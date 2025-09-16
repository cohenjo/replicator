package position

import (
	"context"
	"fmt"
	"time"
)

// Position represents a stream position that can be serialized and restored
type Position interface {
	// Serialize converts the position to a byte array for storage
	Serialize() ([]byte, error)
	
	// Deserialize restores the position from a byte array
	Deserialize(data []byte) error
	
	// String returns a human-readable representation of the position
	String() string
	
	// IsValid checks if the position is valid
	IsValid() bool
	
	// Compare compares this position with another position
	// Returns: -1 if this < other, 0 if equal, 1 if this > other
	Compare(other Position) int
}

// Tracker defines the interface for position tracking implementations
type Tracker interface {
	// Save stores the current position with optional metadata
	Save(ctx context.Context, streamID string, position Position, metadata map[string]interface{}) error
	
	// Load retrieves the stored position for a stream
	Load(ctx context.Context, streamID string) (Position, map[string]interface{}, error)
	
	// Delete removes the stored position for a stream
	Delete(ctx context.Context, streamID string) error
	
	// List returns all stored stream positions
	List(ctx context.Context) (map[string]Position, error)
	
	// Close releases any resources held by the tracker
	Close() error
	
	// HealthCheck verifies the tracker is operational
	HealthCheck(ctx context.Context) error
}

// Config holds configuration for position tracking
type Config struct {
	// Type specifies the tracking implementation type
	Type string `json:"type" yaml:"type"`
	
	// StreamID is the unique identifier for the stream
	StreamID string `json:"stream_id" yaml:"stream_id"`
	
	// FileConfig for file-based tracking
	FileConfig *FileConfig `json:"file,omitempty" yaml:"file,omitempty"`
	
	// AzureConfig for Azure Storage tracking
	AzureConfig *AzureStorageConfig `json:"azure,omitempty" yaml:"azure,omitempty"`
	
	// DatabaseConfig for database-based tracking
	DatabaseConfig *DatabaseConfig `json:"database,omitempty" yaml:"database,omitempty"`
	
	// MongoConfig for MongoDB-specific tracking
	MongoConfig *MongoConfig `json:"mongo,omitempty" yaml:"mongo,omitempty"`
	
	// UpdateInterval for periodic position saves
	UpdateInterval time.Duration `json:"update_interval" yaml:"update_interval"`
	
	// EnableCompression for position data compression
	EnableCompression bool `json:"enable_compression" yaml:"enable_compression"`
	
	// RetryAttempts for failed operations
	RetryAttempts int `json:"retry_attempts" yaml:"retry_attempts"`
	
	// RetryDelay between retry attempts
	RetryDelay time.Duration `json:"retry_delay" yaml:"retry_delay"`
}

// FileConfig for file-based position tracking
type FileConfig struct {
	// Directory where position files are stored
	Directory string `json:"directory" yaml:"directory"`
	
	// FilePermissions for created files (e.g., 0644)
	FilePermissions uint32 `json:"file_permissions" yaml:"file_permissions"`
	
	// EnableBackup creates backup files on update
	EnableBackup bool `json:"enable_backup" yaml:"enable_backup"`
	
	// BackupCount maximum number of backup files to keep
	BackupCount int `json:"backup_count" yaml:"backup_count"`
	
	// SyncInterval for fsync operations
	SyncInterval time.Duration `json:"sync_interval" yaml:"sync_interval"`
}

// AzureStorageConfig for Azure Storage based position tracking
type AzureStorageConfig struct {
	// AccountName for the Azure Storage account
	AccountName string `json:"account_name" yaml:"account_name"`
	
	// AccountKey for authentication (alternative to connection string)
	AccountKey string `json:"account_key,omitempty" yaml:"account_key,omitempty"`
	
	// ConnectionString for Azure Storage (alternative to account name/key)
	ConnectionString string `json:"connection_string,omitempty" yaml:"connection_string,omitempty"`
	
	// Container name for storing position blobs
	ContainerName string `json:"container_name" yaml:"container_name"`
	
	// BlobPrefix for position blob names
	BlobPrefix string `json:"blob_prefix" yaml:"blob_prefix"`
	
	// EnableVersioning for blob versioning
	EnableVersioning bool `json:"enable_versioning" yaml:"enable_versioning"`
	
	// UseLeaseBasedLocking for concurrent access protection
	UseLeaseBasedLocking bool `json:"use_lease_based_locking" yaml:"use_lease_based_locking"`
	
	// LeaseTimeout for lease-based locking
	LeaseTimeout time.Duration `json:"lease_timeout" yaml:"lease_timeout"`
}

// DatabaseConfig for database-based position tracking
type DatabaseConfig struct {
	// Type specifies the database type (mysql, postgres, mongodb, etc.)
	Type string `json:"type" yaml:"type"`
	
	// ConnectionString for database connection
	ConnectionString string `json:"connection_string" yaml:"connection_string"`
	
	// TableName for storing positions (SQL databases)
	TableName string `json:"table_name" yaml:"table_name"`
	
	// CollectionName for storing positions (NoSQL databases)
	CollectionName string `json:"collection_name" yaml:"collection_name"`
	
	// Schema for the position table/collection
	Schema string `json:"schema,omitempty" yaml:"schema,omitempty"`
	
	// EnableAutoMigration creates table/collection if it doesn't exist
	EnableAutoMigration bool `json:"enable_auto_migration" yaml:"enable_auto_migration"`
	
	// UseTransactions for atomic position updates
	UseTransactions bool `json:"use_transactions" yaml:"use_transactions"`
	
	// ConnectionPoolSize for database connections
	ConnectionPoolSize int `json:"connection_pool_size" yaml:"connection_pool_size"`
	
	// ConnectionTimeout for database operations
	ConnectionTimeout time.Duration `json:"connection_timeout" yaml:"connection_timeout"`
	
	// MongoConfig for MongoDB-specific configuration
	MongoConfig *MongoConfig `json:"mongo,omitempty" yaml:"mongo,omitempty"`
}

// Metadata represents additional information stored with positions
type Metadata struct {
	// Timestamp when the position was saved
	Timestamp time.Time `json:"timestamp"`
	
	// Version of the position format
	Version string `json:"version"`
	
	// StreamType (mysql, mongodb, kafka, etc.)
	StreamType string `json:"stream_type"`
	
	// HostInfo about the source system
	HostInfo map[string]string `json:"host_info,omitempty"`
	
	// Custom metadata fields
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// PositionRecord represents a stored position with metadata
type PositionRecord struct {
	// StreamID uniquely identifies the stream
	StreamID string `json:"stream_id"`
	
	// PositionData is the serialized position
	PositionData []byte `json:"position_data"`
	
	// Metadata contains additional information
	Metadata Metadata `json:"metadata"`
	
	// CreatedAt timestamp
	CreatedAt time.Time `json:"created_at"`
	
	// UpdatedAt timestamp
	UpdatedAt time.Time `json:"updated_at"`
}

// NewTracker creates a new position tracker based on configuration
func NewTracker(config *Config) (Tracker, error) {
	switch config.Type {
	case "file":
		return NewFileTracker(config.FileConfig)
	case "azure":
		return nil, fmt.Errorf("azure storage tracker not available in this build")
	case "database":
		return NewDatabaseTracker(config.DatabaseConfig)
	case "mongodb", "mongo":
		return NewMongoTracker(config.MongoConfig)
	default:
		return nil, ErrUnsupportedTrackerType
	}
}

// Factory function type for custom tracker implementations
type TrackerFactory func(config interface{}) (Tracker, error)

// Registry for custom tracker implementations
var trackerRegistry = make(map[string]TrackerFactory)

// RegisterTracker registers a custom tracker implementation
func RegisterTracker(name string, factory TrackerFactory) {
	trackerRegistry[name] = factory
}

// CreateTracker creates a tracker using registered factories
func CreateTracker(name string, config interface{}) (Tracker, error) {
	factory, exists := trackerRegistry[name]
	if !exists {
		return nil, ErrUnsupportedTrackerType
	}
	return factory(config)
}