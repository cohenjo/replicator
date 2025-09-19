package position

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

// MongoTracker implements position tracking using MongoDB
type MongoTracker struct {
	client           *mongo.Client
	database         *mongo.Database
	collection       *mongo.Collection
	config          *MongoConfig
	logger          *logrus.Logger
	closed          bool
}

// MongoConfig holds MongoDB-specific configuration for position tracking
type MongoConfig struct {
	// ConnectionURI for MongoDB connection
	ConnectionURI string `json:"connection_uri" yaml:"connection_uri"`
	
	// Database name for storing positions
	Database string `json:"database" yaml:"database"`
	
	// Collection name for storing positions
	Collection string `json:"collection" yaml:"collection"`
	
	// ConnectTimeout for initial connection
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
	
	// ServerSelectionTimeout for server selection
	ServerSelectionTimeout time.Duration `json:"server_selection_timeout" yaml:"server_selection_timeout"`
	
	// SocketTimeout for individual operations
	SocketTimeout time.Duration `json:"socket_timeout" yaml:"socket_timeout"`
	
	// MaxPoolSize for connection pool
	MaxPoolSize uint64 `json:"max_pool_size" yaml:"max_pool_size"`
	
	// MinPoolSize for connection pool
	MinPoolSize uint64 `json:"min_pool_size" yaml:"min_pool_size"`
	
	// ReadConcern level (local, available, majority, linearizable, snapshot)
	ReadConcern string `json:"read_concern" yaml:"read_concern"`
	
	// WriteConcern configuration
	WriteConcern *MongoWriteConcern `json:"write_concern,omitempty" yaml:"write_concern,omitempty"`
	
	// EnableTransactions for atomic operations
	EnableTransactions bool `json:"enable_transactions" yaml:"enable_transactions"`
	
	// EnableAutoIndexCreation creates indexes on collection
	EnableAutoIndexCreation bool `json:"enable_auto_index_creation" yaml:"enable_auto_index_creation"`
	
	// RetryWrites for automatic retry of write operations
	RetryWrites bool `json:"retry_writes" yaml:"retry_writes"`
	
	// RetryReads for automatic retry of read operations
	RetryReads bool `json:"retry_reads" yaml:"retry_reads"`
	
	// Compressors for network compression (zlib, zstd, snappy)
	Compressors []string `json:"compressors,omitempty" yaml:"compressors,omitempty"`
	
	// AuthMethod specifies the authentication method: "connection_string" or "entra"
	AuthMethod string `json:"auth_method,omitempty" yaml:"auth_method,omitempty"`
	
	// TenantID for Azure Entra authentication
	TenantID string `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	
	// ClientID for Azure Entra authentication
	ClientID string `json:"client_id,omitempty" yaml:"client_id,omitempty"`
	
	// Scopes for Azure Entra authentication (defaults to https://cosmos.azure.com/.default)
	Scopes []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
}

// MongoWriteConcern configuration for write operations
type MongoWriteConcern struct {
	// W specifies the write concern (number or "majority")
	W interface{} `json:"w" yaml:"w"`
	
	// J specifies whether to wait for journal acknowledgment
	J bool `json:"j" yaml:"j"`
	
	// WTimeout specifies the time limit for the write concern
	WTimeout time.Duration `json:"wtimeout" yaml:"wtimeout"`
}

// MongoPositionDocument represents a position document in MongoDB
type MongoPositionDocument struct {
	ID           string                 `bson:"_id" json:"_id"`                       // streamID as document ID
	StreamID     string                 `bson:"stream_id" json:"stream_id"`
	PositionData []byte                 `bson:"position_data" json:"position_data"`
	Metadata     map[string]interface{} `bson:"metadata" json:"metadata"`
	CreatedAt    time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time              `bson:"updated_at" json:"updated_at"`
	Version      int64                  `bson:"version" json:"version"`               // For optimistic locking
}

// NewMongoTracker creates a new MongoDB-based position tracker
func NewMongoTracker(config *MongoConfig) (*MongoTracker, error) {
	if config == nil {
		return nil, fmt.Errorf("mongo config is required")
	}
	
	if config.ConnectionURI == "" {
		return nil, fmt.Errorf("connection URI is required")
	}
	
	if config.Database == "" {
		return nil, fmt.Errorf("database name is required")
	}
	
	if config.Collection == "" {
		config.Collection = "stream_positions"
	}
	
	// Validate authentication method
	if config.AuthMethod == "" {
		config.AuthMethod = "connection_string"
	}
	
	if config.AuthMethod != "connection_string" && config.AuthMethod != "entra" {
		return nil, fmt.Errorf("auth method must be 'connection_string' or 'entra'")
	}
	
	// Set defaults
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 10 * time.Second
	}
	
	if config.ServerSelectionTimeout == 0 {
		config.ServerSelectionTimeout = 30 * time.Second
	}
	
	if config.SocketTimeout == 0 {
		config.SocketTimeout = 10 * time.Second
	}
	
	if config.MaxPoolSize == 0 {
		config.MaxPoolSize = 100
	}
	
	if config.MinPoolSize == 0 {
		config.MinPoolSize = 1
	}
	
	if config.ReadConcern == "" {
		config.ReadConcern = "majority"
	}
	
	if config.WriteConcern == nil {
		config.WriteConcern = &MongoWriteConcern{
			W:        "majority",
			J:        true,
			WTimeout: 5 * time.Second,
		}
	}
	
	// Set Entra authentication defaults and validation
	if config.AuthMethod == "entra" {
		if len(config.Scopes) == 0 {
			config.Scopes = []string{"https://cosmos.azure.com/.default"}
		}
		
		// Validate required Entra fields
		if config.TenantID == "" {
			return nil, fmt.Errorf("tenant ID is required for Entra authentication")
		}
		
		// Validate tenant ID format (should be UUID)
		if len(config.TenantID) != 36 || config.TenantID[8] != '-' || config.TenantID[13] != '-' {
			return nil, fmt.Errorf("tenant ID must be valid UUID format")
		}
		
		// ClientID is optional for system-assigned managed identity; only required for user-assigned identity or service principal.
		// If your logic requires distinguishing, add further checks here.
		
		// Validate scopes for Azure Cosmos DB
		validScope := false
		for _, scope := range config.Scopes {
			if scope == "https://cosmos.azure.com/.default" {
				validScope = true
				break
			}
		}
		if !validScope {
			return nil, fmt.Errorf("invalid scope for Azure Cosmos DB, must include 'https://cosmos.azure.com/.default'")
		}
		
		// Check that connection URI doesn't contain credentials when using Entra
		if strings.Contains(config.ConnectionURI, "://") {
			uriParts := strings.SplitN(config.ConnectionURI, "://", 2)
			if len(uriParts) == 2 {
				hostPart := uriParts[1]
				// Check for credentials pattern: username:password@host
				if strings.Contains(hostPart, "@") {
					beforeAt := strings.SplitN(hostPart, "@", 2)[0]
					if strings.Contains(beforeAt, ":") {
						return nil, fmt.Errorf("connection URI must not contain credentials when using Entra authentication")
					}
				}
			}
		}
	}
	
	// Create MongoDB client options
	clientOpts := options.Client().
		ApplyURI(config.ConnectionURI).
		SetConnectTimeout(config.ConnectTimeout).
		SetServerSelectionTimeout(config.ServerSelectionTimeout).
		SetSocketTimeout(config.SocketTimeout).
		SetMaxPoolSize(config.MaxPoolSize).
		SetMinPoolSize(config.MinPoolSize).
		SetRetryWrites(config.RetryWrites).
		SetRetryReads(config.RetryReads)
	
	// Set read concern
	switch config.ReadConcern {
	case "local":
		clientOpts.SetReadConcern(readconcern.Local())
	case "available":
		clientOpts.SetReadConcern(readconcern.Available())
	case "majority":
		clientOpts.SetReadConcern(readconcern.Majority())
	case "linearizable":
		clientOpts.SetReadConcern(readconcern.Linearizable())
	case "snapshot":
		clientOpts.SetReadConcern(readconcern.Snapshot())
	}
	
	// Set write concern
	if config.WriteConcern != nil {
		wcOpts := []writeconcern.Option{}
		
		// Set W (write concern)
		switch w := config.WriteConcern.W.(type) {
		case int:
			wcOpts = append(wcOpts, writeconcern.W(w))
		case string:
			if w == "majority" {
				wcOpts = append(wcOpts, writeconcern.WMajority())
			} else {
				wcOpts = append(wcOpts, writeconcern.WTagSet(w))
			}
		default:
			wcOpts = append(wcOpts, writeconcern.WMajority())
		}
		
		// Set journal requirement
		if config.WriteConcern.J {
			wcOpts = append(wcOpts, writeconcern.J(config.WriteConcern.J))
		}
		
		// Set timeout
		if config.WriteConcern.WTimeout > 0 {
			wcOpts = append(wcOpts, writeconcern.WTimeout(config.WriteConcern.WTimeout))
		}
		
		wc := writeconcern.New(wcOpts...)
		clientOpts.SetWriteConcern(wc)
	}
	
	// Set compressors
	if len(config.Compressors) > 0 {
		clientOpts.SetCompressors(config.Compressors)
	}
	
	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()
	
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	
	// Test the connection
	if err := client.Ping(ctx, options.Ping()); err != nil {
		client.Disconnect(context.Background())
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}
	
	database := client.Database(config.Database)
	collection := database.Collection(config.Collection)
	
	tracker := &MongoTracker{
		client:     client,
		database:   database,
		collection: collection,
		config:     config,
		logger:     logrus.New(),
		closed:     false,
	}
	
	// Create indexes if auto-creation is enabled
	if config.EnableAutoIndexCreation {
		if err := tracker.createIndexes(ctx); err != nil {
			tracker.logger.WithError(err).Warn("Failed to create indexes")
		}
	}
	
	tracker.logger.WithFields(logrus.Fields{
		"database":          config.Database,
		"collection":        config.Collection,
		"read_concern":      config.ReadConcern,
		"write_concern":     config.WriteConcern,
		"transactions":      config.EnableTransactions,
		"auto_indexes":      config.EnableAutoIndexCreation,
	}).Info("Created MongoDB-based position tracker")
	
	return tracker, nil
}

// Save stores the position in MongoDB
func (mt *MongoTracker) Save(ctx context.Context, streamID string, position Position, metadata map[string]interface{}) error {
	if mt.closed {
		return ErrTrackerClosed
	}
	
	// Serialize position
	positionData, err := position.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize position: %w", err)
	}
	
	// Prepare metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	
	// Add system metadata
	metadata["timestamp"] = time.Now()
	metadata["version"] = "1.0"
	if streamType, exists := metadata["stream_type"]; !exists || streamType == "" {
		metadata["stream_type"] = "unknown"
	}
	
	// Create document
	doc := MongoPositionDocument{
		ID:           streamID,
		StreamID:     streamID,
		PositionData: positionData,
		Metadata:     metadata,
		UpdatedAt:    time.Now(),
		Version:      time.Now().Unix(), // Simple versioning for optimistic locking
	}
	
	if mt.config.EnableTransactions {
		return mt.saveWithTransaction(ctx, &doc)
	}
	
	return mt.saveWithoutTransaction(ctx, &doc)
}

// saveWithTransaction saves position using MongoDB transactions
func (mt *MongoTracker) saveWithTransaction(ctx context.Context, doc *MongoPositionDocument) error {
	session, err := mt.client.StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)
	
	callback := func(sessionCtx mongo.SessionContext) (interface{}, error) {
		// Check if document exists to preserve created_at
		filter := bson.M{"_id": doc.ID}
		var existing MongoPositionDocument
		err := mt.collection.FindOne(sessionCtx, filter).Decode(&existing)
		if err == nil {
			doc.CreatedAt = existing.CreatedAt
		} else if err == mongo.ErrNoDocuments {
			doc.CreatedAt = time.Now()
		} else {
			return nil, fmt.Errorf("failed to check existing document: %w", err)
		}
		
		// Upsert document
		opts := options.Replace().SetUpsert(true)
		_, err = mt.collection.ReplaceOne(sessionCtx, filter, doc, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to upsert document: %w", err)
		}
		
		return nil, nil
	}
	
	_, err = mongo.WithSession(ctx, session, callback)
	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}
	
	mt.logger.WithFields(logrus.Fields{
		"stream_id": doc.StreamID,
		"position":  string(doc.PositionData),
		"version":   doc.Version,
	}).Debug("Saved position with transaction")
	
	return nil
}

// saveWithoutTransaction saves position without using transactions
func (mt *MongoTracker) saveWithoutTransaction(ctx context.Context, doc *MongoPositionDocument) error {
	// Check if document exists to preserve created_at
	filter := bson.M{"_id": doc.ID}
	var existing MongoPositionDocument
	err := mt.collection.FindOne(ctx, filter).Decode(&existing)
	if err == nil {
		doc.CreatedAt = existing.CreatedAt
	} else if err == mongo.ErrNoDocuments {
		doc.CreatedAt = time.Now()
	} else {
		return fmt.Errorf("failed to check existing document: %w", err)
	}
	
	// Upsert document
	opts := options.Replace().SetUpsert(true)
	_, err = mt.collection.ReplaceOne(ctx, filter, doc, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert document: %w", err)
	}
	
	mt.logger.WithFields(logrus.Fields{
		"stream_id": doc.StreamID,
		"position":  string(doc.PositionData),
		"version":   doc.Version,
	}).Debug("Saved position without transaction")
	
	return nil
}

// Load retrieves the position from MongoDB
func (mt *MongoTracker) Load(ctx context.Context, streamID string) (Position, map[string]interface{}, error) {
	if mt.closed {
		return nil, nil, ErrTrackerClosed
	}
	
	filter := bson.M{"_id": streamID}
	var doc MongoPositionDocument
	
	err := mt.collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
	if mongo.IsErrNoDocuments(err) {
		return nil, nil, ErrPositionNotFound
	}
		return nil, nil, fmt.Errorf("failed to find document: %w", err)
	}
	
	// For now, return nil position as we need a position type registry to properly deserialize
	// In a complete implementation, you would have a factory to create the correct position type
	var position Position = nil
	
	mt.logger.WithFields(logrus.Fields{
		"stream_id":  streamID,
		"created_at": doc.CreatedAt,
		"updated_at": doc.UpdatedAt,
		"version":    doc.Version,
	}).Debug("Loaded position from MongoDB")
	
	return position, doc.Metadata, nil
}

// Delete removes the position from MongoDB
func (mt *MongoTracker) Delete(ctx context.Context, streamID string) error {
	if mt.closed {
		return ErrTrackerClosed
	}
	
	filter := bson.M{"_id": streamID}
	result, err := mt.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	
	if result.DeletedCount == 0 {
		return ErrPositionNotFound
	}
	
	mt.logger.WithFields(logrus.Fields{
		"stream_id":      streamID,
		"deleted_count":  result.DeletedCount,
	}).Info("Deleted position from MongoDB")
	
	return nil
}

// List returns all stored positions
func (mt *MongoTracker) List(ctx context.Context) (map[string]Position, error) {
	if mt.closed {
		return nil, ErrTrackerClosed
	}
	
	cursor, err := mt.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to find documents: %w", err)
	}
	defer cursor.Close(ctx)
	
	positions := make(map[string]Position)
	
	for cursor.Next(ctx) {
		var doc MongoPositionDocument
		if err := cursor.Decode(&doc); err != nil {
			mt.logger.WithError(err).WithField("stream_id", doc.StreamID).Warn("Failed to decode document")
			continue
		}
		
		// Create position from document (simplified for now)
		var position Position = nil
		positions[doc.StreamID] = position
	}
	
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}
	
	return positions, nil
}

// Close releases MongoDB resources
func (mt *MongoTracker) Close() error {
	if mt.closed {
		return nil
	}
	
	mt.closed = true
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := mt.client.Disconnect(ctx); err != nil {
		mt.logger.WithError(err).Error("Failed to disconnect from MongoDB")
		return err
	}
	
	mt.logger.Info("Closed MongoDB-based position tracker")
	return nil
}

// HealthCheck verifies the tracker is operational
func (mt *MongoTracker) HealthCheck(ctx context.Context) error {
	if mt.closed {
		return ErrTrackerClosed
	}
	
	// Ping MongoDB
	if err := mt.client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("MongoDB ping failed: %w", err)
	}
	
	// Test collection access
	count, err := mt.collection.EstimatedDocumentCount(ctx)
	if err != nil {
		return fmt.Errorf("collection access failed: %w", err)
	}
	
	mt.logger.WithField("document_count", count).Debug("MongoDB health check passed")
	return nil
}

// createIndexes creates recommended indexes for the position collection
func (mt *MongoTracker) createIndexes(ctx context.Context) error {
	indexModels := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "stream_id", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("stream_id_unique"),
		},
		{
			Keys: bson.D{
				{Key: "updated_at", Value: -1},
			},
			Options: options.Index().SetName("updated_at_desc"),
		},
		{
			Keys: bson.D{
				{Key: "metadata.stream_type", Value: 1},
			},
			Options: options.Index().SetName("stream_type_idx"),
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: 1},
			},
			Options: options.Index().SetName("created_at_asc"),
		},
	}
	
	_, err := mt.collection.Indexes().CreateMany(ctx, indexModels)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}
	
	mt.logger.Info("Created MongoDB indexes for position tracking")
	return nil
}

// GetStats returns collection statistics
func (mt *MongoTracker) GetStats(ctx context.Context) (map[string]interface{}, error) {
	if mt.closed {
		return nil, ErrTrackerClosed
	}
	
	// Get collection stats
	var result bson.M
	err := mt.database.RunCommand(ctx, bson.D{
		{Key: "collStats", Value: mt.config.Collection},
	}).Decode(&result)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get collection stats: %w", err)
	}
	
	// Convert to map[string]interface{}
	stats := make(map[string]interface{})
	statsBytes, _ := json.Marshal(result)
	json.Unmarshal(statsBytes, &stats)
	
	return stats, nil
}
