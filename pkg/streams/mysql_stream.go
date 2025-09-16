package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/rs/zerolog/log"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/cohenjo/replicator/pkg/models"
)

// MySQLStream implements the models.Stream interface for MySQL binlog replication
type MySQLStream struct {
	config       config.StreamConfig
	syncer       *replication.BinlogSyncer
	streamer     *replication.BinlogStreamer
	state        models.StreamState
	metrics      models.ReplicationMetrics
	eventChannel chan<- events.RecordEvent
	stopChan     chan struct{}
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewMySQLStream creates a new MySQL stream instance
func NewMySQLStream(streamConfig config.StreamConfig, eventChannel chan<- events.RecordEvent) (*MySQLStream, error) {
	// Validate configuration
	if streamConfig.Source.Type != "mysql" {
		return nil, fmt.Errorf("invalid source type for MySQL stream: %s", streamConfig.Source.Type)
	}

	return &MySQLStream{
		config:       streamConfig,
		eventChannel: eventChannel,
		stopChan:     make(chan struct{}),
		state: models.StreamState{
			Name:   streamConfig.Name,
			Status: config.StreamStatusStopped,
		},
		metrics: models.ReplicationMetrics{
			StreamName: streamConfig.Name,
		},
	}, nil
}

// Start begins the replication stream
func (s *MySQLStream) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status == config.StreamStatusRunning {
		return fmt.Errorf("stream is already running")
	}

	log.Info().Str("stream", s.config.Name).Msg("Starting MySQL stream")

	// Create context for this stream
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Setup MySQL binlog syncer
	if err := s.setupSyncer(); err != nil {
		s.state.Status = config.StreamStatusError
		lastError := err.Error()
		s.state.LastError = &lastError
		return fmt.Errorf("failed to setup MySQL syncer: %w", err)
	}

	// Start binlog streaming
	if err := s.startStreaming(); err != nil {
		s.state.Status = config.StreamStatusError
		lastError := err.Error()
		s.state.LastError = &lastError
		return fmt.Errorf("failed to start binlog streaming: %w", err)
	}

	// Update state
	s.state.Status = config.StreamStatusRunning
	now := time.Now()
	s.state.StartedAt = &now
	s.state.LastError = nil
	s.metrics.LastProcessedTime = time.Now()

	// Start processing events in background
	go s.processEvents()

	log.Info().Str("stream", s.config.Name).Msg("MySQL stream started successfully")
	return nil
}

// Stop gracefully stops the replication stream
func (s *MySQLStream) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status == config.StreamStatusStopped {
		return nil
	}

	log.Info().Str("stream", s.config.Name).Msg("Stopping MySQL stream")

	// Cancel context to stop processing
	if s.cancel != nil {
		s.cancel()
	}

	// Close binlog streamer
	if s.streamer != nil {
		// Binlog streamer doesn't have a public Close method
		// It will be closed when the syncer is closed
	}

	// Close syncer
	if s.syncer != nil {
		s.syncer.Close()
	}

	// Update state
	s.state.Status = config.StreamStatusStopped
	now := time.Now()
	s.state.StoppedAt = &now

	log.Info().Str("stream", s.config.Name).Msg("MySQL stream stopped")
	return nil
}

// Pause temporarily pauses the replication stream
func (s *MySQLStream) Pause(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status != config.StreamStatusRunning {
		return fmt.Errorf("stream is not running")
	}

	s.state.Status = config.StreamStatusPaused
	log.Info().Str("stream", s.config.Name).Msg("MySQL stream paused")
	return nil
}

// Resume resumes a paused replication stream
func (s *MySQLStream) Resume(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status != config.StreamStatusPaused {
		return fmt.Errorf("stream is not paused")
	}

	s.state.Status = config.StreamStatusRunning
	log.Info().Str("stream", s.config.Name).Msg("MySQL stream resumed")
	return nil
}

// GetState returns the current state of the stream
func (s *MySQLStream) GetState() models.StreamState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// GetConfig returns the configuration of the stream
func (s *MySQLStream) GetConfig() config.StreamConfig {
	return s.config
}

// GetMetrics returns current metrics for the stream
func (s *MySQLStream) GetMetrics() models.ReplicationMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Calculate events per second
	if s.metrics.EventsProcessed > 0 && !s.metrics.LastProcessedTime.IsZero() {
		duration := time.Since(s.metrics.LastProcessedTime)
		if duration > 0 {
			s.metrics.EventsPerSecond = float64(s.metrics.EventsProcessed) / duration.Seconds()
		}
	}

	return s.metrics
}

// SetCheckpoint updates the stream checkpoint
func (s *MySQLStream) SetCheckpoint(checkpoint map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store checkpoint (implementation depends on requirements)
	log.Debug().Interface("checkpoint", checkpoint).Str("stream", s.config.Name).Msg("Checkpoint updated")
	return nil
}

// GetCheckpoint returns the current checkpoint
func (s *MySQLStream) GetCheckpoint() (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return current checkpoint (implementation depends on requirements)
	return make(map[string]interface{}), nil
}

// setupSyncer configures the MySQL binlog syncer
func (s *MySQLStream) setupSyncer() error {
	// Build MySQL config
	cfg := replication.BinlogSyncerConfig{
		ServerID: 100, // Should be configurable
		Flavor:   "mysql",
		Host:     s.config.Source.Host,
		Port:     uint16(s.config.Source.Port),
		User:     s.config.Source.Username,
		Password: s.config.Source.Password,
	}

	s.syncer = replication.NewBinlogSyncer(cfg)
	return nil
}

// startStreaming starts the binlog streaming
func (s *MySQLStream) startStreaming() error {
	// Start from beginning by default (should be configurable)
	pos := mysql.Position{Name: "", Pos: 4}
	
	streamer, err := s.syncer.StartSync(pos)
	if err != nil {
		return fmt.Errorf("failed to start binlog sync: %w", err)
	}

	s.streamer = streamer
	return nil
}

// processEvents processes binlog events
func (s *MySQLStream) processEvents() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("stream", s.config.Name).Msg("Panic in event processing")
			s.mu.Lock()
			s.state.Status = config.StreamStatusError
			lastError := fmt.Sprintf("panic: %v", r)
			s.state.LastError = &lastError
			s.mu.Unlock()
		}
	}()

	log.Info().Str("stream", s.config.Name).Msg("Starting MySQL event processing")

	for {
		select {
		case <-s.ctx.Done():
			log.Info().Str("stream", s.config.Name).Msg("MySQL event processing stopped")
			return
		default:
			// Check if stream is paused
			s.mu.RLock()
			isPaused := s.state.Status == config.StreamStatusPaused
			s.mu.RUnlock()

			if isPaused {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Get next event with timeout
			ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
			ev, err := s.streamer.GetEvent(ctx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded {
					continue // Normal timeout, just continue
				}
				log.Error().Err(err).Str("stream", s.config.Name).Msg("Failed to get binlog event")
				s.mu.Lock()
				s.metrics.ErrorCount++
				s.mu.Unlock()
				continue
			}

			// Process the event
			log.Debug().Str("stream", s.config.Name).Str("eventType", ev.Header.EventType.String()).Msg("Processing binlog event")
			if err := s.processBinlogEvent(ev); err != nil {
				log.Error().Err(err).Str("stream", s.config.Name).Msg("Failed to process binlog event")
				s.mu.Lock()
				s.metrics.ErrorCount++
				s.mu.Unlock()
				continue
			}

			// Update metrics
			s.mu.Lock()
			s.metrics.EventsProcessed++
			s.metrics.LastProcessedTime = time.Now()
			s.mu.Unlock()
		}
	}
}

// processBinlogEvent processes a single binlog event
func (s *MySQLStream) processBinlogEvent(ev *replication.BinlogEvent) error {
	// Log the event type for debugging
	log.Debug().
		Str("stream", s.config.Name).
		Str("eventType", fmt.Sprintf("%v", ev.Header.EventType)).
		Msg("Processing binlog event")

	switch e := ev.Event.(type) {
	case *replication.RowsEvent:
		return s.processRowsEvent(e, ev.Header.EventType)
	case *replication.QueryEvent:
		return s.processQueryEvent(e)
	default:
		// Ignore other event types for now
		// log.Debug().Str("stream", s.config.Name).Interface("event",e).Msg("Ignoring non-row event")
		return nil
	}
}

// processRowsEvent processes row-level changes (INSERT, UPDATE, DELETE)
func (s *MySQLStream) processRowsEvent(ev *replication.RowsEvent, eventType replication.EventType) error {
	log.Info().
		Str("stream", s.config.Name).
		Str("schema", string(ev.Table.Schema)).
		Str("table", string(ev.Table.Table)).
		Msg("Processing row event")

	// Determine the action based on the event type
	var eventTypeStr string
	var action string
	
	// The eventType contains the event type information
	switch eventType {
	case replication.WRITE_ROWS_EVENTv0, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:
		eventTypeStr = "insert"
		action = "insert"
	case replication.UPDATE_ROWS_EVENTv0, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2:
		eventTypeStr = "update"
		action = "update"
	case replication.DELETE_ROWS_EVENTv0, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
		eventTypeStr = "delete"
		action = "delete"
	default:
		log.Debug().
			Str("stream", s.config.Name).
			Str("eventType", fmt.Sprintf("%v", eventType)).
			Msg("Ignoring unknown row event type")
		return nil
	}
	
	log.Debug().
		Str("stream", s.config.Name).
		Str("mysqlEventType", eventTypeStr).
		Msg("Row event type detected")

	log.Debug().
		Str("stream", s.config.Name).
		Str("action", action).
		Msg("Action determined for row event")

	// Filter by database if specified
	if s.config.Source.Database != "" && string(ev.Table.Schema) != s.config.Source.Database {
		log.Info().Str("stream", s.config.Name).Str("schema", string(ev.Table.Schema)).Msg("Skipping event due to database filter")
		return nil
	}

	// Filter by table if specified
	if tableFilter := s.getTableFromConfig(); tableFilter != "" && string(ev.Table.Table) != tableFilter {
		log.Info().Str("stream", s.config.Name).Str("table", string(ev.Table.Table)).Msg("Skipping event due to table filter")
		return nil
	}

	// Process each row
	for _, row := range ev.Rows {
		if err := s.processRow(action, string(ev.Table.Schema), string(ev.Table.Table), row); err != nil {
			log.Error().Err(err).Str("stream", s.config.Name).Msg("Failed to process row")
			return err
		}
	}

	return nil
}

// processQueryEvent processes DDL and other query events
func (s *MySQLStream) processQueryEvent(ev *replication.QueryEvent) error {
	// For now, we'll ignore query events
	// In a full implementation, we'd process DDL changes here
	log.Debug().
		Str("stream", s.config.Name).
		Str("query", string(ev.Query)).
		Msg("Query event received (ignored)")
	return nil
}

// processRow processes a single row change
func (s *MySQLStream) processRow(action, schema, table string, row []interface{}) error {
	// Convert row data to JSON
	data, err := json.Marshal(row)
	if err != nil {
		log.Error().Err(err).Str("stream", s.config.Name).Msg("Failed to marshal row data")
		return err
	}

	// Create replication event using the existing RecordEvent structure
	recordEvent := events.RecordEvent{
		Action:     action,
		Schema:     schema,
		Collection: table,
		Data:       data,
	}

	log.Debug().
		Str("stream", s.config.Name).
		Str("action", action).
		Str("table", table).
		Msg("Processed row event")

	// Send to event channel (non-blocking)
	select {
	case s.eventChannel <- recordEvent:
		log.Debug().
			Str("stream", s.config.Name).
			Str("action", action).
			Str("table", table).
			Msg("Event sent to processing pipeline")
	default:
		log.Warn().
			Str("stream", s.config.Name).
			Msg("Event channel full, dropping event")
		s.mu.Lock()
		s.metrics.ErrorCount++
		s.mu.Unlock()
	}

	return nil
}

// getTableFromConfig extracts table name from configuration
func (s *MySQLStream) getTableFromConfig() string {
	if s.config.Source.Options != nil {
		if table, ok := s.config.Source.Options["table"].(string); ok {
			return table
		}
	}
	return ""
}