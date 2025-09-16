package estuary

import (
	"context"
	"fmt"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
)

// DatabaseDestination defines the interface for database destination operations
type DatabaseDestination interface {
	// Connection management
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	IsConnected() bool
	Ping(ctx context.Context) error
	
	// Data operations
	Write(ctx context.Context, data []byte, table string) error
	WriteBatch(ctx context.Context, batch []DestinationRecord) error
	CreateTable(ctx context.Context, schema TableSchema) error
	DropTable(ctx context.Context, tableName string) error
	TruncateTable(ctx context.Context, tableName string) error
	
	// Schema operations
	GetSchema(ctx context.Context, tableName string) (*TableSchema, error)
	UpdateSchema(ctx context.Context, tableName string, schema TableSchema) error
	ListTables(ctx context.Context) ([]string, error)
	
	// Transaction management
	BeginTransaction(ctx context.Context) (Transaction, error)
	
	// Monitoring and health
	GetHealth(ctx context.Context) (*HealthStatus, error)
	GetMetrics(ctx context.Context) (*DestinationMetrics, error)
	
	// Configuration
	GetConfig() config.TargetConfig
	ValidateConfig() error
	
	// Cleanup
	Close() error
}

// DestinationFactory creates database destination instances
type DestinationFactory interface {
	CreateDestination(ctx context.Context, config config.TargetConfig) (DatabaseDestination, error)
	GetSupportedTypes() []string
	ValidateConfig(config config.TargetConfig) error
}

// DestinationRegistry manages multiple database destinations
type DestinationRegistry interface {
	RegisterDestination(name string, destination DatabaseDestination) error
	GetDestination(name string) (DatabaseDestination, error)
	RemoveDestination(name string) error
	ListDestinations() []string
	GetDestinationByType(destinationType string) (DatabaseDestination, error)
	
	// Bulk operations
	WriteToAll(ctx context.Context, data []byte, table string) error
	WriteToMultiple(ctx context.Context, destinationNames []string, data []byte, table string) error
	
	// Health monitoring
	CheckAllHealth(ctx context.Context) (map[string]*HealthStatus, error)
	GetAllMetrics(ctx context.Context) (map[string]*DestinationMetrics, error)
}

// Transaction represents a database transaction
type Transaction interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Write(ctx context.Context, data []byte, table string) error
	WriteBatch(ctx context.Context, batch []DestinationRecord) error
	IsActive() bool
	GetID() string
}

// DestinationRecord represents a single record to be written to a destination
type DestinationRecord struct {
	Table     string                 `json:"table"`
	Operation string                 `json:"operation"` // INSERT, UPDATE, DELETE, UPSERT
	Data      map[string]interface{} `json:"data"`
	Key       map[string]interface{} `json:"key,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// TableSchema represents the schema of a database table
type TableSchema struct {
	Name        string                 `json:"name"`
	Columns     []ColumnDefinition     `json:"columns"`
	PrimaryKey  []string               `json:"primary_key"`
	Indexes     []IndexDefinition      `json:"indexes,omitempty"`
	Constraints []ConstraintDefinition `json:"constraints,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ColumnDefinition represents a table column
type ColumnDefinition struct {
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	Nullable     bool        `json:"nullable"`
	DefaultValue interface{} `json:"default_value,omitempty"`
	MaxLength    *int        `json:"max_length,omitempty"`
	Precision    *int        `json:"precision,omitempty"`
	Scale        *int        `json:"scale,omitempty"`
}

// IndexDefinition represents a table index
type IndexDefinition struct {
	Name     string   `json:"name"`
	Columns  []string `json:"columns"`
	Unique   bool     `json:"unique"`
	Type     string   `json:"type,omitempty"` // BTREE, HASH, etc.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ConstraintDefinition represents a table constraint
type ConstraintDefinition struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"` // FOREIGN_KEY, CHECK, UNIQUE, etc.
	Columns    []string `json:"columns"`
	Expression string   `json:"expression,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// HealthStatus represents the health status of a destination
type HealthStatus struct {
	Status       string                 `json:"status"` // HEALTHY, DEGRADED, UNHEALTHY
	Message      string                 `json:"message,omitempty"`
	LastCheck    time.Time              `json:"last_check"`
	ResponseTime time.Duration          `json:"response_time"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// DestinationMetrics contains performance and usage metrics
type DestinationMetrics struct {
	// Connection metrics
	TotalConnections    int64         `json:"total_connections"`
	ActiveConnections   int64         `json:"active_connections"`
	ConnectionErrors    int64         `json:"connection_errors"`
	AverageConnectTime  time.Duration `json:"average_connect_time"`
	
	// Write metrics
	TotalWrites         int64         `json:"total_writes"`
	SuccessfulWrites    int64         `json:"successful_writes"`
	FailedWrites        int64         `json:"failed_writes"`
	AverageWriteTime    time.Duration `json:"average_write_time"`
	BytesWritten        int64         `json:"bytes_written"`
	RecordsWritten      int64         `json:"records_written"`
	
	// Transaction metrics
	TotalTransactions   int64         `json:"total_transactions"`
	CommittedTransactions int64       `json:"committed_transactions"`
	RolledBackTransactions int64      `json:"rolled_back_transactions"`
	AverageTransactionTime time.Duration `json:"average_transaction_time"`
	
	// Error metrics
	TotalErrors         int64         `json:"total_errors"`
	LastError           string        `json:"last_error,omitempty"`
	LastErrorTime       *time.Time    `json:"last_error_time,omitempty"`
	
	// Performance metrics
	Throughput          float64       `json:"throughput"` // records per second
	Latency             time.Duration `json:"latency"`
	
	// Timestamp
	LastUpdated         time.Time     `json:"last_updated"`
}

// ConnectionPoolConfig configures connection pooling for destinations
type ConnectionPoolConfig struct {
	MaxConnections     int           `json:"max_connections"`
	MinConnections     int           `json:"min_connections"`
	MaxIdleTime        time.Duration `json:"max_idle_time"`
	MaxLifetime        time.Duration `json:"max_lifetime"`
	ConnectionTimeout  time.Duration `json:"connection_timeout"`
	ValidationQuery    string        `json:"validation_query,omitempty"`
}

// RetryConfig configures retry behavior for failed operations
type RetryConfig struct {
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor"`
	RetryableErrors []string      `json:"retryable_errors,omitempty"`
}

// BatchConfig configures batching behavior for write operations
type BatchConfig struct {
	BatchSize        int           `json:"batch_size"`
	BatchTimeout     time.Duration `json:"batch_timeout"`
	MaxBatchSize     int           `json:"max_batch_size"`
	FlushInterval    time.Duration `json:"flush_interval"`
	EnableBatching   bool          `json:"enable_batching"`
}

// DestinationManager provides high-level destination management
type DestinationManager interface {
	// Factory management
	RegisterFactory(destinationType string, factory DestinationFactory) error
	GetFactory(destinationType string) (DestinationFactory, error)
	
	// Destination lifecycle
	CreateDestination(ctx context.Context, name string, config config.TargetConfig) error
	GetDestination(name string) (DatabaseDestination, error)
	RemoveDestination(ctx context.Context, name string) error
	
	// Configuration management
	UpdateDestinationConfig(ctx context.Context, name string, config config.TargetConfig) error
	GetDestinationConfig(name string) (*config.TargetConfig, error)
	
	// Health and monitoring
	MonitorDestinations(ctx context.Context, interval time.Duration) error
	GetDestinationHealth(name string) (*HealthStatus, error)
	GetDestinationMetrics(name string) (*DestinationMetrics, error)
	
	// Bulk operations
	WriteToDestination(ctx context.Context, destinationName string, records []DestinationRecord) error
	WriteToMultipleDestinations(ctx context.Context, destinationNames []string, records []DestinationRecord) error
	
	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool
}

// DataTransformer transforms data before writing to destinations
type DataTransformer interface {
	Transform(ctx context.Context, input []byte, targetSchema TableSchema) ([]DestinationRecord, error)
	ValidateSchema(schema TableSchema) error
	GetSupportedFormats() []string
}

// SchemaEvolution handles schema changes in destinations
type SchemaEvolution interface {
	CompareSchemas(current, new TableSchema) (*SchemaComparison, error)
	GenerateMigration(comparison *SchemaComparison) (*SchemaMigration, error)
	ApplyMigration(ctx context.Context, destination DatabaseDestination, migration *SchemaMigration) error
	ValidateMigration(migration *SchemaMigration) error
}

// SchemaComparison represents the differences between two schemas
type SchemaComparison struct {
	TableName      string                `json:"table_name"`
	AddedColumns   []ColumnDefinition    `json:"added_columns,omitempty"`
	RemovedColumns []ColumnDefinition    `json:"removed_columns,omitempty"`
	ModifiedColumns []ColumnModification `json:"modified_columns,omitempty"`
	AddedIndexes   []IndexDefinition     `json:"added_indexes,omitempty"`
	RemovedIndexes []IndexDefinition     `json:"removed_indexes,omitempty"`
	HasChanges     bool                  `json:"has_changes"`
}

// ColumnModification represents changes to a column
type ColumnModification struct {
	Name        string           `json:"name"`
	OldColumn   ColumnDefinition `json:"old_column"`
	NewColumn   ColumnDefinition `json:"new_column"`
	ChangeType  string           `json:"change_type"` // TYPE_CHANGE, NULL_CHANGE, DEFAULT_CHANGE, etc.
}

// SchemaMigration represents a schema migration
type SchemaMigration struct {
	ID          string                `json:"id"`
	TableName   string                `json:"table_name"`
	Operations  []MigrationOperation  `json:"operations"`
	CreatedAt   time.Time             `json:"created_at"`
	Description string                `json:"description,omitempty"`
}

// MigrationOperation represents a single migration operation
type MigrationOperation struct {
	Type        string                 `json:"type"` // ADD_COLUMN, DROP_COLUMN, MODIFY_COLUMN, etc.
	Parameters  map[string]interface{} `json:"parameters"`
	SQL         string                 `json:"sql,omitempty"`
	Description string                 `json:"description,omitempty"`
}

// DestinationError represents errors that occur during destination operations
type DestinationError struct {
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Operation   string                 `json:"operation"`
	Destination string                 `json:"destination"`
	Timestamp   time.Time              `json:"timestamp"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Cause       error                  `json:"-"`
}

// Error implements the error interface
func (e *DestinationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s in %s operation on %s: %v", e.Code, e.Message, e.Operation, e.Destination, e.Cause)
	}
	return fmt.Sprintf("[%s] %s in %s operation on %s", e.Code, e.Message, e.Operation, e.Destination)
}

// Unwrap returns the underlying cause
func (e *DestinationError) Unwrap() error {
	return e.Cause
}

// Common error codes
const (
	ErrCodeConnectionFailed    = "CONNECTION_FAILED"
	ErrCodeWriteFailed         = "WRITE_FAILED"
	ErrCodeSchemaNotFound      = "SCHEMA_NOT_FOUND"
	ErrCodeInvalidSchema       = "INVALID_SCHEMA"
	ErrCodeTransactionFailed   = "TRANSACTION_FAILED"
	ErrCodeTableNotFound       = "TABLE_NOT_FOUND"
	ErrCodeInvalidConfig       = "INVALID_CONFIG"
	ErrCodeDestinationNotFound = "DESTINATION_NOT_FOUND"
	ErrCodeUnsupportedOperation = "UNSUPPORTED_OPERATION"
)

// Helper functions for creating destination errors
func NewConnectionError(destination, message string, cause error) *DestinationError {
	return &DestinationError{
		Code:        ErrCodeConnectionFailed,
		Message:     message,
		Operation:   "connect",
		Destination: destination,
		Timestamp:   time.Now(),
		Cause:       cause,
	}
}

func NewWriteError(destination, table, message string, cause error) *DestinationError {
	return &DestinationError{
		Code:        ErrCodeWriteFailed,
		Message:     message,
		Operation:   "write",
		Destination: destination,
		Timestamp:   time.Now(),
		Details:     map[string]interface{}{"table": table},
		Cause:       cause,
	}
}

func NewSchemaError(destination, table, message string, cause error) *DestinationError {
	return &DestinationError{
		Code:        ErrCodeSchemaNotFound,
		Message:     message,
		Operation:   "schema",
		Destination: destination,
		Timestamp:   time.Now(),
		Details:     map[string]interface{}{"table": table},
		Cause:       cause,
	}
}

func NewTransactionError(destination, message string, cause error) *DestinationError {
	return &DestinationError{
		Code:        ErrCodeTransactionFailed,
		Message:     message,
		Operation:   "transaction",
		Destination: destination,
		Timestamp:   time.Now(),
		Cause:       cause,
	}
}