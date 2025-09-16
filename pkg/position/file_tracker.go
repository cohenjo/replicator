package position

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// FileTracker implements position tracking using local files
type FileTracker struct {
	config    *FileConfig
	directory string
	mutex     sync.RWMutex
	logger    *logrus.Logger
	closed    bool
}

// NewFileTracker creates a new file-based position tracker
func NewFileTracker(config *FileConfig) (*FileTracker, error) {
	if config == nil {
		return nil, fmt.Errorf("file config is required")
	}
	
	if config.Directory == "" {
		return nil, fmt.Errorf("directory is required")
	}
	
	// Set defaults
	if config.FilePermissions == 0 {
		config.FilePermissions = 0644
	}
	
	if config.BackupCount == 0 {
		config.BackupCount = 5
	}
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(config.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", config.Directory, err)
	}
	
	tracker := &FileTracker{
		config:    config,
		directory: config.Directory,
		logger:    logrus.New(),
		closed:    false,
	}
	
	tracker.logger.WithFields(logrus.Fields{
		"directory":      config.Directory,
		"backup_enabled": config.EnableBackup,
		"backup_count":   config.BackupCount,
	}).Info("Created file-based position tracker")
	
	return tracker, nil
}

// Save stores the position to a file
func (ft *FileTracker) Save(ctx context.Context, streamID string, position Position, metadata map[string]interface{}) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()
	
	if ft.closed {
		return ErrTrackerClosed
	}
	
	// Serialize position
	positionData, err := position.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize position: %w", err)
	}
	
	// Create position record
	record := PositionRecord{
		StreamID:     streamID,
		PositionData: positionData,
		Metadata: Metadata{
			Timestamp:  time.Now(),
			Version:    "1.0",
			StreamType: getStreamTypeFromMetadata(metadata),
			Custom:     metadata,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Check if file exists to preserve created timestamp
	filePath := ft.getPositionFilePath(streamID)
	if existingRecord, err := ft.loadPositionRecord(filePath); err == nil {
		record.CreatedAt = existingRecord.CreatedAt
	}
	
	// Marshal to JSON
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal position record: %w", err)
	}
	
	// Create backup if enabled
	if ft.config.EnableBackup {
		if err := ft.createBackup(streamID); err != nil {
			ft.logger.WithError(err).Warn("Failed to create backup")
		}
	}
	
	// Write to temporary file first, then rename for atomicity
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, fs.FileMode(ft.config.FilePermissions)); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	
	// Sync if configured
	if ft.config.SyncInterval > 0 {
		if file, err := os.OpenFile(tempPath, os.O_RDWR, 0); err == nil {
			file.Sync()
			file.Close()
		}
	}
	
	// Atomic rename
	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}
	
	ft.logger.WithFields(logrus.Fields{
		"stream_id": streamID,
		"position":  position.String(),
		"file_path": filePath,
	}).Debug("Saved position to file")
	
	return nil
}

// Load retrieves the position from a file
func (ft *FileTracker) Load(ctx context.Context, streamID string) (Position, map[string]interface{}, error) {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()
	
	if ft.closed {
		return nil, nil, ErrTrackerClosed
	}
	
	filePath := ft.getPositionFilePath(streamID)
	record, err := ft.loadPositionRecord(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrPositionNotFound
		}
		return nil, nil, fmt.Errorf("failed to load position record: %w", err)
	}
	
	// Try to determine position type from metadata and deserialize accordingly
	// This is a simplified approach - in practice you'd have a position type registry
	// For now, we return nil position as a placeholder
	var position Position = nil
	
	ft.logger.WithFields(logrus.Fields{
		"stream_id":  streamID,
		"created_at": record.CreatedAt,
		"updated_at": record.UpdatedAt,
	}).Debug("Loaded position from file")
	
	return position, record.Metadata.Custom, nil
}

// Delete removes the position file
func (ft *FileTracker) Delete(ctx context.Context, streamID string) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()
	
	if ft.closed {
		return ErrTrackerClosed
	}
	
	filePath := ft.getPositionFilePath(streamID)
	
	// Remove main file
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove position file: %w", err)
	}
	
	// Remove backups
	if ft.config.EnableBackup {
		ft.removeBackups(streamID)
	}
	
	ft.logger.WithFields(logrus.Fields{
		"stream_id": streamID,
		"file_path": filePath,
	}).Info("Deleted position file")
	
	return nil
}

// List returns all stored positions
func (ft *FileTracker) List(ctx context.Context) (map[string]Position, error) {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()
	
	if ft.closed {
		return nil, ErrTrackerClosed
	}
	
	positions := make(map[string]Position)
	
	// Read all position files
	entries, err := os.ReadDir(ft.directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		
		// Skip backup and temporary files
		if strings.Contains(entry.Name(), ".backup") || strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		
		// Extract stream ID from filename
		streamID := strings.TrimSuffix(entry.Name(), ".json")
		
		filePath := filepath.Join(ft.directory, entry.Name())
		_, err := ft.loadPositionRecord(filePath)
		if err != nil {
			ft.logger.WithError(err).WithField("file", entry.Name()).Warn("Failed to load position record")
			continue
		}
		
		// Create position from record (simplified)
		var position Position
		positions[streamID] = position
	}
	
	return positions, nil
}

// Close releases resources (no-op for file tracker)
func (ft *FileTracker) Close() error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()
	
	ft.closed = true
	ft.logger.Info("Closed file-based position tracker")
	
	return nil
}

// HealthCheck verifies the tracker is operational
func (ft *FileTracker) HealthCheck(ctx context.Context) error {
	if ft.closed {
		return ErrTrackerClosed
	}
	
	// Check if directory exists and is writable
	testFile := filepath.Join(ft.directory, ".health_check")
	if err := os.WriteFile(testFile, []byte("test"), fs.FileMode(ft.config.FilePermissions)); err != nil {
		return fmt.Errorf("directory not writable: %w", err)
	}
	
	os.Remove(testFile)
	return nil
}

// getPositionFilePath returns the file path for a stream's position
func (ft *FileTracker) getPositionFilePath(streamID string) string {
	filename := fmt.Sprintf("%s.json", streamID)
	return filepath.Join(ft.directory, filename)
}

// loadPositionRecord loads a position record from file
func (ft *FileTracker) loadPositionRecord(filePath string) (*PositionRecord, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	var record PositionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal position record: %w", err)
	}
	
	return &record, nil
}

// createBackup creates a backup of the current position file
func (ft *FileTracker) createBackup(streamID string) error {
	filePath := ft.getPositionFilePath(streamID)
	
	// Check if main file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // No file to backup
	}
	
	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.backup.%s", filePath, timestamp)
	
	// Copy file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(backupPath, data, fs.FileMode(ft.config.FilePermissions)); err != nil {
		return err
	}
	
	// Clean up old backups
	return ft.cleanupOldBackups(streamID)
}

// cleanupOldBackups removes old backup files beyond the configured limit
func (ft *FileTracker) cleanupOldBackups(streamID string) error {
	pattern := fmt.Sprintf("%s.json.backup.*", streamID)
	matches, err := filepath.Glob(filepath.Join(ft.directory, pattern))
	if err != nil {
		return err
	}
	
	// Sort by modification time (newest first)
	sort.Slice(matches, func(i, j int) bool {
		info1, _ := os.Stat(matches[i])
		info2, _ := os.Stat(matches[j])
		return info1.ModTime().After(info2.ModTime())
	})
	
	// Remove excess backups
	if len(matches) > ft.config.BackupCount {
		for i := ft.config.BackupCount; i < len(matches); i++ {
			os.Remove(matches[i])
		}
	}
	
	return nil
}

// removeBackups removes all backup files for a stream
func (ft *FileTracker) removeBackups(streamID string) {
	pattern := fmt.Sprintf("%s.json.backup.*", streamID)
	matches, _ := filepath.Glob(filepath.Join(ft.directory, pattern))
	for _, match := range matches {
		os.Remove(match)
	}
}

// getStreamTypeFromMetadata extracts stream type from metadata
func getStreamTypeFromMetadata(metadata map[string]interface{}) string {
	if metadata == nil {
		return "unknown"
	}
	
	if streamType, ok := metadata["stream_type"].(string); ok {
		return streamType
	}
	
	return "unknown"
}