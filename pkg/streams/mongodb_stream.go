package streams

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/rs/zerolog/log"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/cohenjo/replicator/pkg/models"
)

// MongoDBStream implements the models.Stream interface for MongoDB replication
type MongoDBStream struct {
	config       config.StreamConfig
	client       *mongo.Client
	changeStream *mongo.ChangeStream
	state        models.StreamState
	metrics      models.ReplicationMetrics
	eventChannel chan<- events.RecordEvent
	stopChan     chan struct{}
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewMongoDBStream creates a new MongoDB stream instance
func NewMongoDBStream(streamConfig config.StreamConfig, eventChannel chan<- events.RecordEvent) (*MongoDBStream, error) {
	// Validate configuration
	if streamConfig.Source.Type != "mongodb" {
		return nil, fmt.Errorf("invalid source type for MongoDB stream: %s", streamConfig.Source.Type)
	}

	return &MongoDBStream{
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
func (s *MongoDBStream) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status == config.StreamStatusRunning {
		return fmt.Errorf("stream is already running")
	}

	log.Info().Str("stream", s.config.Name).Msg("Starting MongoDB stream")

	// Create context for this stream
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Connect to MongoDB
	if err := s.connect(); err != nil {
		s.state.Status = config.StreamStatusError
		lastError := err.Error()
		s.state.LastError = &lastError
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Create change stream
	if err := s.createChangeStream(); err != nil {
		s.state.Status = config.StreamStatusError
		lastError := err.Error()
		s.state.LastError = &lastError
		return fmt.Errorf("failed to create change stream: %w", err)
	}

	// Update state
	s.state.Status = config.StreamStatusRunning
	now := time.Now()
	s.state.StartedAt = &now
	s.state.LastError = nil
	s.metrics.LastProcessedTime = time.Now()

	// Start processing events in background
	go s.processEvents()

	log.Info().Str("stream", s.config.Name).Msg("MongoDB stream started successfully")
	return nil
}

// Stop gracefully stops the replication stream
func (s *MongoDBStream) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status == config.StreamStatusStopped {
		return nil
	}

	log.Info().Str("stream", s.config.Name).Msg("Stopping MongoDB stream")

	// Cancel context to stop processing
	if s.cancel != nil {
		s.cancel()
	}

	// Close change stream
	if s.changeStream != nil {
		if err := s.changeStream.Close(ctx); err != nil {
			log.Warn().Err(err).Str("stream", s.config.Name).Msg("Error closing change stream")
		}
	}

	// Disconnect from MongoDB
	if s.client != nil {
		if err := s.client.Disconnect(ctx); err != nil {
			log.Warn().Err(err).Str("stream", s.config.Name).Msg("Error disconnecting from MongoDB")
		}
	}

	// Update state
	s.state.Status = config.StreamStatusStopped
	now := time.Now()
	s.state.StoppedAt = &now

	log.Info().Str("stream", s.config.Name).Msg("MongoDB stream stopped")
	return nil
}

// Pause temporarily pauses the replication stream
func (s *MongoDBStream) Pause(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status != config.StreamStatusRunning {
		return fmt.Errorf("stream is not running")
	}

	s.state.Status = config.StreamStatusPaused
	log.Info().Str("stream", s.config.Name).Msg("MongoDB stream paused")
	return nil
}

// Resume resumes a paused replication stream
func (s *MongoDBStream) Resume(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status != config.StreamStatusPaused {
		return fmt.Errorf("stream is not paused")
	}

	s.state.Status = config.StreamStatusRunning
	log.Info().Str("stream", s.config.Name).Msg("MongoDB stream resumed")
	return nil
}

// GetState returns the current state of the stream
func (s *MongoDBStream) GetState() models.StreamState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// GetConfig returns the configuration of the stream
func (s *MongoDBStream) GetConfig() config.StreamConfig {
	return s.config
}

// GetMetrics returns current metrics for the stream
func (s *MongoDBStream) GetMetrics() models.ReplicationMetrics {
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
func (s *MongoDBStream) SetCheckpoint(checkpoint map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store checkpoint (implementation depends on requirements)
	log.Debug().Interface("checkpoint", checkpoint).Str("stream", s.config.Name).Msg("Checkpoint updated")
	return nil
}

// GetCheckpoint returns the current checkpoint
func (s *MongoDBStream) GetCheckpoint() (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return current checkpoint (implementation depends on requirements)
	return make(map[string]interface{}), nil
}

// connect establishes connection to MongoDB
func (s *MongoDBStream) connect() error {
	// Use URI if provided, otherwise build connection string
	var connectionString string
	if s.config.Source.URI != "" {
		connectionString = s.config.Source.URI
	} else {
		// Build connection string from individual components
		// For MongoDB, authentication should be against admin database
		authDB := "admin"
		if s.config.Source.Options != nil {
			if authDatabase, ok := s.config.Source.Options["authDatabase"].(string); ok && authDatabase != "" {
				authDB = authDatabase
			}
		}
		
		connectionString = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=%s",
			s.config.Source.Username,
			s.config.Source.Password,
			s.config.Source.Host,
			s.config.Source.Port,
			s.config.Source.Database,
			authDB,
		)
	}

	// Create client options
	clientOptions := options.Client().ApplyURI(connectionString)

	// Connect to MongoDB
	client, err := mongo.Connect(s.ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Test the connection
	if err := client.Ping(s.ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	s.client = client
	log.Info().Str("stream", s.config.Name).Msg("Connected to MongoDB")
	return nil
}

// createChangeStream creates a MongoDB change stream
func (s *MongoDBStream) createChangeStream() error {
	database := s.client.Database(s.config.Source.Database)
	
	// Use collection if specified, otherwise watch entire database
	var changeStream *mongo.ChangeStream
	var err error

	pipeline := mongo.Pipeline{}
	options := options.ChangeStream().SetFullDocument(options.UpdateLookup)

	if collection := s.getCollectionFromConfig(); collection != "" {
		// Watch specific collection
		coll := database.Collection(collection)
		changeStream, err = coll.Watch(s.ctx, pipeline, options)
	} else {
		// Watch entire database
		changeStream, err = database.Watch(s.ctx, pipeline, options)
	}

	if err != nil {
		return fmt.Errorf("failed to create change stream: %w", err)
	}

	s.changeStream = changeStream
	log.Info().Str("stream", s.config.Name).Msg("Change stream created")
	return nil
}

// processEvents processes change stream events
func (s *MongoDBStream) processEvents() {
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

	log.Info().Str("stream", s.config.Name).Msg("Starting event processing")

	for s.changeStream.Next(s.ctx) {
		// Check if stream is paused
		s.mu.RLock()
		isPaused := s.state.Status == config.StreamStatusPaused
		s.mu.RUnlock()

		if isPaused {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Decode the change event
		var changeEvent bson.M
		if err := s.changeStream.Decode(&changeEvent); err != nil {
			log.Error().Err(err).Str("stream", s.config.Name).Msg("Failed to decode change event")
			s.mu.Lock()
			s.metrics.ErrorCount++
			s.mu.Unlock()
			continue
		}

		// Process the event
		if err := s.processChangeEvent(changeEvent); err != nil {
			log.Error().Err(err).Str("stream", s.config.Name).Msg("Failed to process change event")
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

	// Check for errors
	if err := s.changeStream.Err(); err != nil {
		log.Error().Err(err).Str("stream", s.config.Name).Msg("Change stream error")
		s.mu.Lock()
		s.state.Status = config.StreamStatusError
		lastError := err.Error()
		s.state.LastError = &lastError
		s.mu.Unlock()
	}

	log.Info().Str("stream", s.config.Name).Msg("Event processing stopped")
}

// processChangeEvent processes a single change event
func (s *MongoDBStream) processChangeEvent(changeEvent bson.M) error {
	// Extract basic event information
	operationType, _ := changeEvent["operationType"].(string)
	fullDocument, _ := changeEvent["fullDocument"].(bson.M)

	// Convert full document to JSON bytes
	var data []byte
	var err error
	if fullDocument != nil {
		data, err = bson.MarshalExtJSON(fullDocument, true, false)
		if err != nil {
			log.Error().Err(err).Str("stream", s.config.Name).Msg("Failed to marshal full document")
			return err
		}
	}

	// Create replication event using the existing RecordEvent structure
	recordEvent := events.RecordEvent{
		Action:     operationType,
		Schema:     s.config.Source.Database,
		Collection: s.getCollectionFromEvent(changeEvent),
		Data:       data,
	}

	// Send to event channel (non-blocking)
	select {
	case s.eventChannel <- recordEvent:
		log.Debug().
			Str("stream", s.config.Name).
			Str("operation", operationType).
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

// getCollectionFromEvent extracts collection name from change event
func (s *MongoDBStream) getCollectionFromEvent(changeEvent bson.M) string {
	if ns, ok := changeEvent["ns"].(bson.M); ok {
		if coll, ok := ns["coll"].(string); ok {
			return coll
		}
	}
	return s.getCollectionFromConfig()
}

// getCollectionFromConfig extracts collection name from configuration
func (s *MongoDBStream) getCollectionFromConfig() string {
	if s.config.Source.Options != nil {
		if collection, ok := s.config.Source.Options["collection"].(string); ok {
			return collection
		}
	}
	return ""
}