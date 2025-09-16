package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/rs/zerolog/log"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/cohenjo/replicator/pkg/models"
)

// PostgreSQLStream implements the models.Stream interface for PostgreSQL logical replication
type PostgreSQLStream struct {
	config       config.StreamConfig
	conn         *pgconn.PgConn
	state        models.StreamState
	metrics      models.ReplicationMetrics
	eventChannel chan<- events.RecordEvent
	stopChan     chan struct{}
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	slotName     string
	publication  string
}

// NewPostgreSQLStream creates a new PostgreSQL stream instance
func NewPostgreSQLStream(streamConfig config.StreamConfig, eventChannel chan<- events.RecordEvent) (*PostgreSQLStream, error) {
	// Validate configuration
	if streamConfig.Source.Type != "postgresql" {
		return nil, fmt.Errorf("invalid source type for PostgreSQL stream: %s", streamConfig.Source.Type)
	}

	// Generate unique slot name if not provided
	slotName := "replicator_slot"
	if streamConfig.Source.Options != nil {
		if slot, ok := streamConfig.Source.Options["slot_name"].(string); ok && slot != "" {
			slotName = slot
		}
	}

	// Generate publication name if not provided
	publication := "replicator_publication"
	if streamConfig.Source.Options != nil {
		if pub, ok := streamConfig.Source.Options["publication"].(string); ok && pub != "" {
			publication = pub
		}
	}

	return &PostgreSQLStream{
		config:       streamConfig,
		eventChannel: eventChannel,
		stopChan:     make(chan struct{}),
		slotName:     slotName,
		publication:  publication,
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
func (s *PostgreSQLStream) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status == config.StreamStatusRunning {
		return fmt.Errorf("stream is already running")
	}

	log.Info().Str("stream", s.config.Name).Msg("Starting PostgreSQL stream")

	// Create context for this stream
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Setup PostgreSQL connection
	if err := s.setupConnection(); err != nil {
		s.state.Status = config.StreamStatusError
		lastError := err.Error()
		s.state.LastError = &lastError
		return fmt.Errorf("failed to setup PostgreSQL connection: %w", err)
	}

	// Setup replication slot and publication
	if err := s.setupReplication(); err != nil {
		s.state.Status = config.StreamStatusError
		lastError := err.Error()
		s.state.LastError = &lastError
		return fmt.Errorf("failed to setup replication: %w", err)
	}

	// Start replication streaming
	if err := s.startReplication(); err != nil {
		s.state.Status = config.StreamStatusError
		lastError := err.Error()
		s.state.LastError = &lastError
		return fmt.Errorf("failed to start replication: %w", err)
	}

	// Update state
	s.state.Status = config.StreamStatusRunning
	now := time.Now()
	s.state.StartedAt = &now
	s.state.LastError = nil
	s.metrics.LastProcessedTime = time.Now()

	// Start processing events in background
	go s.processEvents()

	log.Info().Str("stream", s.config.Name).Msg("PostgreSQL stream started successfully")
	return nil
}

// Stop gracefully stops the replication stream
func (s *PostgreSQLStream) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status == config.StreamStatusStopped {
		return nil
	}

	log.Info().Str("stream", s.config.Name).Msg("Stopping PostgreSQL stream")

	// Cancel context to stop processing
	if s.cancel != nil {
		s.cancel()
	}

	// Close connection
	if s.conn != nil {
		s.conn.Close(ctx)
	}

	// Update state
	s.state.Status = config.StreamStatusStopped
	now := time.Now()
	s.state.StoppedAt = &now

	log.Info().Str("stream", s.config.Name).Msg("PostgreSQL stream stopped")
	return nil
}

// Pause temporarily pauses the replication stream
func (s *PostgreSQLStream) Pause(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status != config.StreamStatusRunning {
		return fmt.Errorf("stream is not running")
	}

	s.state.Status = config.StreamStatusPaused
	log.Info().Str("stream", s.config.Name).Msg("PostgreSQL stream paused")
	return nil
}

// Resume resumes a paused replication stream
func (s *PostgreSQLStream) Resume(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status != config.StreamStatusPaused {
		return fmt.Errorf("stream is not paused")
	}

	s.state.Status = config.StreamStatusRunning
	log.Info().Str("stream", s.config.Name).Msg("PostgreSQL stream resumed")
	return nil
}

// GetState returns the current state of the stream
func (s *PostgreSQLStream) GetState() models.StreamState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// GetConfig returns the configuration of the stream
func (s *PostgreSQLStream) GetConfig() config.StreamConfig {
	return s.config
}

// GetMetrics returns current metrics for the stream
func (s *PostgreSQLStream) GetMetrics() models.ReplicationMetrics {
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
func (s *PostgreSQLStream) SetCheckpoint(checkpoint map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store checkpoint (implementation depends on requirements)
	log.Debug().Interface("checkpoint", checkpoint).Str("stream", s.config.Name).Msg("Checkpoint updated")
	return nil
}

// GetCheckpoint returns the current checkpoint
func (s *PostgreSQLStream) GetCheckpoint() (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return current checkpoint (implementation depends on requirements)
	return make(map[string]interface{}), nil
}

// setupConnection establishes connection to PostgreSQL
func (s *PostgreSQLStream) setupConnection() error {
	connString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		s.config.Source.Host,
		s.config.Source.Port,
		s.config.Source.Username,
		s.config.Source.Password,
		s.config.Source.Database,
	)

	conn, err := pgconn.Connect(s.ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	s.conn = conn
	return nil
}

// setupReplication sets up replication slot and publication
func (s *PostgreSQLStream) setupReplication() error {
	// Create replication slot if it doesn't exist
	if err := s.createReplicationSlot(); err != nil {
		return fmt.Errorf("failed to create replication slot: %w", err)
	}

	// Create publication if it doesn't exist
	if err := s.createPublication(); err != nil {
		return fmt.Errorf("failed to create publication: %w", err)
	}

	return nil
}

// createReplicationSlot creates a logical replication slot
func (s *PostgreSQLStream) createReplicationSlot() error {
	// Check if slot already exists
	slotExists, err := s.slotExists()
	if err != nil {
		return err
	}

	if slotExists {
		log.Info().Str("slot", s.slotName).Msg("Replication slot already exists")
		return nil
	}

	// Create the slot
	_, err = pglogrepl.CreateReplicationSlot(
		s.ctx,
		s.conn,
		s.slotName,
		"pgoutput",
		pglogrepl.CreateReplicationSlotOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to create replication slot: %w", err)
	}

	log.Info().Str("slot", s.slotName).Msg("Replication slot created")
	return nil
}

// slotExists checks if the replication slot exists
func (s *PostgreSQLStream) slotExists() (bool, error) {
	result := s.conn.Exec(s.ctx, "SELECT 1 FROM pg_replication_slots WHERE slot_name = '"+s.slotName+"'")
	rows, err := result.ReadAll()
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

// createPublication creates a publication for replication
func (s *PostgreSQLStream) createPublication() error {
	// Check if publication exists
	pubExists, err := s.publicationExists()
	if err != nil {
		return err
	}

	if pubExists {
		log.Info().Str("publication", s.publication).Msg("Publication already exists")
		return nil
	}

	// Create publication for all tables by default
	tableSpec := "ALL TABLES"
	if tableFilter := s.getTableFromConfig(); tableFilter != "" {
		tableSpec = fmt.Sprintf("TABLE %s", tableFilter)
	}

	query := fmt.Sprintf("CREATE PUBLICATION %s FOR %s", s.publication, tableSpec)
	result := s.conn.Exec(s.ctx, query)
	_, err = result.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to create publication: %w", err)
	}

	log.Info().Str("publication", s.publication).Msg("Publication created")
	return nil
}

// publicationExists checks if the publication exists
func (s *PostgreSQLStream) publicationExists() (bool, error) {
	result := s.conn.Exec(s.ctx, "SELECT 1 FROM pg_publication WHERE pubname = '"+s.publication+"'")
	rows, err := result.ReadAll()
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

// startReplication starts the logical replication stream
func (s *PostgreSQLStream) startReplication() error {
	options := pglogrepl.StartReplicationOptions{
		PluginArgs: []string{
			"proto_version '1'",
			fmt.Sprintf("publication_names '%s'", s.publication),
		},
	}

	err := pglogrepl.StartReplication(s.ctx, s.conn, s.slotName, pglogrepl.LSN(0), options)
	if err != nil {
		return fmt.Errorf("failed to start replication: %w", err)
	}

	return nil
}

// processEvents processes logical replication events
func (s *PostgreSQLStream) processEvents() {
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

	log.Info().Str("stream", s.config.Name).Msg("Starting PostgreSQL event processing")

	for {
		select {
		case <-s.ctx.Done():
			log.Info().Str("stream", s.config.Name).Msg("PostgreSQL event processing stopped")
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

			// Receive message with timeout
			ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
			msg, err := s.conn.ReceiveMessage(ctx)
			cancel()

			if err != nil {
				if pgconn.Timeout(err) {
					continue // Normal timeout, just continue
				}
				log.Error().Err(err).Str("stream", s.config.Name).Msg("Failed to receive message")
				s.mu.Lock()
				s.metrics.ErrorCount++
				s.mu.Unlock()
				continue
			}

			// Process the message
			if err := s.processMessage(msg); err != nil {
				log.Error().Err(err).Str("stream", s.config.Name).Msg("Failed to process message")
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

// processMessage processes a single PostgreSQL message
func (s *PostgreSQLStream) processMessage(msg pgproto3.BackendMessage) error {
	switch msg := msg.(type) {
	case *pgproto3.CopyData:
		return s.processCopyData(msg.Data)
	case *pgproto3.ErrorResponse:
		return fmt.Errorf("PostgreSQL error: %s", msg.Message)
	default:
		// Ignore other message types
		return nil
	}
}

// processCopyData processes logical replication data
func (s *PostgreSQLStream) processCopyData(data []byte) error {
	// Parse logical replication message
	logicalMsg, err := pglogrepl.Parse(data)
	if err != nil {
		return fmt.Errorf("failed to parse logical message: %w", err)
	}

	switch msg := logicalMsg.(type) {
	case *pglogrepl.InsertMessage:
		return s.processInsert(msg)
	case *pglogrepl.UpdateMessage:
		return s.processUpdate(msg)
	case *pglogrepl.DeleteMessage:
		return s.processDelete(msg)
	case *pglogrepl.BeginMessage:
		// Transaction begin - might want to track this
		return nil
	case *pglogrepl.CommitMessage:
		// Transaction commit - might want to track this
		return nil
	default:
		// Ignore other message types
		return nil
	}
}

// processInsert processes an INSERT operation
func (s *PostgreSQLStream) processInsert(msg *pglogrepl.InsertMessage) error {
	data, err := s.extractRowData(msg.Tuple)
	if err != nil {
		return err
	}

	return s.sendEvent("insert", msg.RelationID, data)
}

// processUpdate processes an UPDATE operation
func (s *PostgreSQLStream) processUpdate(msg *pglogrepl.UpdateMessage) error {
	data, err := s.extractRowData(msg.NewTuple)
	if err != nil {
		return err
	}

	return s.sendEvent("update", msg.RelationID, data)
}

// processDelete processes a DELETE operation
func (s *PostgreSQLStream) processDelete(msg *pglogrepl.DeleteMessage) error {
	var tuple *pglogrepl.TupleData
	if msg.OldTuple != nil {
		tuple = msg.OldTuple
	} else {
		// For deletes, we might not have the old tuple, use what we have
		tuple = &pglogrepl.TupleData{}
	}

	data, err := s.extractRowData(tuple)
	if err != nil {
		return err
	}

	return s.sendEvent("delete", msg.RelationID, data)
}

// extractRowData extracts data from a tuple
func (s *PostgreSQLStream) extractRowData(tuple *pglogrepl.TupleData) ([]byte, error) {
	if tuple == nil {
		return json.Marshal(map[string]interface{}{})
	}

	// Convert tuple data to map
	rowData := make(map[string]interface{})
	for i, col := range tuple.Columns {
		if col.DataType == pglogrepl.TupleDataTypeNull {
			rowData[fmt.Sprintf("col_%d", i)] = nil
		} else {
			rowData[fmt.Sprintf("col_%d", i)] = string(col.Data)
		}
	}

	return json.Marshal(rowData)
}

// sendEvent sends a replication event to the processing pipeline
func (s *PostgreSQLStream) sendEvent(action string, relationID uint32, data []byte) error {
	// Create replication event
	recordEvent := events.RecordEvent{
		Action:     action,
		Schema:     s.config.Source.Database,
		Collection: fmt.Sprintf("relation_%d", relationID), // In a real implementation, we'd resolve this to table name
		Data:       data,
	}

	// Send to event channel (non-blocking)
	select {
	case s.eventChannel <- recordEvent:
		log.Debug().
			Str("stream", s.config.Name).
			Str("action", action).
			Uint32("relation", relationID).
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
func (s *PostgreSQLStream) getTableFromConfig() string {
	if s.config.Source.Options != nil {
		if table, ok := s.config.Source.Options["table"].(string); ok {
			return table
		}
	}
	return ""
}