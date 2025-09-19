package replicator

import (
"context"
"fmt"
"sync"
"time"

"github.com/cohenjo/replicator/pkg/config"
"github.com/cohenjo/replicator/pkg/events"
"github.com/cohenjo/replicator/pkg/models"
"github.com/sirupsen/logrus"
)

// StreamManager manages multiple replication streams
type StreamManager struct {
	streams      map[string]models.Stream
	streamStates map[string]models.StreamState
	eventChannel chan<- events.RecordEvent
	logger       *logrus.Logger
	mu           sync.RWMutex
}


// StreamManager Implementation
																	
// CreateStream creates a new replication stream
func (sm *StreamManager) CreateStream(config config.StreamConfig) (models.Stream, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if _, exists := sm.streams[config.Name]; exists {
		return nil, fmt.Errorf("stream %s already exists", config.Name)
	}
	
	// TODO: Use stream factory to create appropriate stream type
	return nil, fmt.Errorf("stream creation not implemented")
}

// GetStream retrieves a stream by name
func (sm *StreamManager) GetStream(name string) (models.Stream, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	stream, exists := sm.streams[name]
	return stream, exists
}
																	
// ListStreams returns all configured streams
func (sm *StreamManager) ListStreams() []models.Stream {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	streams := make([]models.Stream, 0, len(sm.streams))
	for _, stream := range sm.streams {
		streams = append(streams, stream)
	}
	
	return streams
}
																	
// DeleteStream removes a stream
func (sm *StreamManager) DeleteStream(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	stream, exists := sm.streams[name]
	if !exists {
		return fmt.Errorf("stream %s not found", name)
	}
	
	// Stop the stream if running
	if stream.GetState().Status == config.StreamStatusRunning {
		if err := stream.Stop(context.Background()); err != nil {
			return fmt.Errorf("failed to stop stream %s: %w", name, err)
		}
	}
	
	delete(sm.streams, name)
	delete(sm.streamStates, name)
	
	return nil
}
																	
// StartAll starts all configured streams
func (sm *StreamManager) StartAll(ctx context.Context) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	for name, stream := range sm.streams {
		if err := stream.Start(ctx); err != nil {
			sm.logger.WithError(err).WithField("stream", name).Error("Failed to start stream")
			return fmt.Errorf("failed to start stream %s: %w", name, err)
		}
		
		sm.logger.WithField("stream", name).Info("Stream started")
	}
	
	return nil
}
																	
// StopAll stops all running streams
func (sm *StreamManager) StopAll(ctx context.Context) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	var errors []error
	
	for name, stream := range sm.streams {
		if stream.GetState().Status == config.StreamStatusRunning {
			if err := stream.Stop(ctx); err != nil {
				sm.logger.WithError(err).WithField("stream", name).Error("Failed to stop stream")
				errors = append(errors, fmt.Errorf("failed to stop stream %s: %w", name, err))
				} else {
					sm.logger.WithField("stream", name).Info("Stream stopped")
				}
			}
		}
		
		if len(errors) > 0 {
			return fmt.Errorf("failed to stop %d streams", len(errors))
		}
		
		return nil
	}
																		
// GetHealthStatus returns overall health status
func (sm *StreamManager) GetHealthStatus() models.HealthStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	status := "healthy"
	streamStates := make(map[string]models.StreamState)
	
	for name, stream := range sm.streams {
		state := stream.GetState()
		streamStates[name] = state
		
		if state.Status == config.StreamStatusError {
			status = "degraded"
		}
	}
	
	return models.HealthStatus{
		Status:      status,
		Timestamp:   time.Now(),
		StreamCount: len(sm.streams),
		Streams:     streamStates,
		Checks:      make(map[string]models.CheckResult),
	}
}
																		