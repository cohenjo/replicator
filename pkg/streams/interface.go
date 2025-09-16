package streams

import (
	"context"
	"fmt"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/models"
)

// StreamFactory creates stream instances based on configuration
type StreamFactory interface {
	// CreateStream creates a new stream instance
	CreateStream(config config.StreamConfig) (models.Stream, error)
	
	// GetSupportedTypes returns supported stream types
	GetSupportedTypes() []string
	
	// ValidateConfig validates stream configuration
	ValidateConfig(config config.StreamConfig) error
}

// StreamRegistry manages stream type registration
type StreamRegistry interface {
	// RegisterFactory registers a stream factory for a specific type
	RegisterFactory(streamType string, factory StreamFactory) error
	
	// GetFactory returns a factory for the given stream type
	GetFactory(streamType string) (StreamFactory, bool)
	
	// GetRegisteredTypes returns all registered stream types
	GetRegisteredTypes() []string
	
	// UnregisterFactory removes a factory registration
	UnregisterFactory(streamType string) error
}

// PositionManager manages stream positions for resumption
type PositionManager interface {
	// SavePosition saves the current position for a stream
	SavePosition(ctx context.Context, streamName string, position map[string]interface{}) error
	
	// LoadPosition loads the last saved position for a stream
	LoadPosition(ctx context.Context, streamName string) (map[string]interface{}, error)
	
	// DeletePosition removes saved position for a stream
	DeletePosition(ctx context.Context, streamName string) error
	
	// ListPositions returns all saved positions
	ListPositions(ctx context.Context) (map[string]map[string]interface{}, error)
}

// HealthChecker performs health checks on streams
type HealthChecker interface {
	// CheckHealth performs health check on a stream
	CheckHealth(ctx context.Context, stream models.Stream) models.CheckResult
	
	// CheckAllStreams performs health check on all streams
	CheckAllStreams(ctx context.Context, streams []models.Stream) map[string]models.CheckResult
}

// StreamEventHandler handles events from streams
type StreamEventHandler interface {
	// HandleEvent processes a single change event
	HandleEvent(ctx context.Context, event models.ChangeEvent) error
	
	// HandleBatch processes a batch of change events
	HandleBatch(ctx context.Context, events []models.ChangeEvent) error
	
	// HandleError processes errors from streams
	HandleError(ctx context.Context, streamName string, err error) error
}

// StreamLifecycleHooks provides hooks for stream lifecycle events
type StreamLifecycleHooks interface {
	// OnStreamStart called when a stream starts
	OnStreamStart(ctx context.Context, streamName string) error
	
	// OnStreamStop called when a stream stops
	OnStreamStop(ctx context.Context, streamName string) error
	
	// OnStreamPause called when a stream is paused
	OnStreamPause(ctx context.Context, streamName string) error
	
	// OnStreamResume called when a stream resumes
	OnStreamResume(ctx context.Context, streamName string) error
	
	// OnStreamError called when a stream encounters an error
	OnStreamError(ctx context.Context, streamName string, err error) error
}

// StreamMonitor monitors stream performance and health
type StreamMonitor interface {
	// StartMonitoring begins monitoring a stream
	StartMonitoring(ctx context.Context, stream models.Stream) error
	
	// StopMonitoring stops monitoring a stream
	StopMonitoring(ctx context.Context, streamName string) error
	
	// GetMetrics returns current metrics for a stream
	GetMetrics(streamName string) (models.ReplicationMetrics, error)
	
	// GetAllMetrics returns metrics for all monitored streams
	GetAllMetrics() map[string]models.ReplicationMetrics
	
	// SetMetricsCollectionInterval sets the metrics collection interval
	SetMetricsCollectionInterval(interval time.Duration)
}

// ConnectionManager manages database connections for streams
type ConnectionManager interface {
	// GetConnection returns a connection for the given configuration
	GetConnection(ctx context.Context, config config.SourceConfig) (interface{}, error)
	
	// CloseConnection closes a connection
	CloseConnection(ctx context.Context, connectionID string) error
	
	// TestConnection tests if a connection is working
	TestConnection(ctx context.Context, config config.SourceConfig) error
	
	// GetActiveConnections returns all active connections
	GetActiveConnections() map[string]interface{}
}

// RetryManager manages retry logic for stream operations
type RetryManager interface {
	// ShouldRetry determines if an operation should be retried
	ShouldRetry(err error, attempt int) bool
	
	// GetDelay returns the delay before the next retry
	GetDelay(attempt int) time.Duration
	
	// Reset resets retry state for a stream
	Reset(streamName string)
	
	// GetRetryState returns current retry state for a stream
	GetRetryState(streamName string) RetryState
}

// RetryState represents the current retry state
type RetryState struct {
	Attempts     int           `json:"attempts"`
	LastAttempt  time.Time     `json:"last_attempt"`
	NextAttempt  time.Time     `json:"next_attempt"`
	LastError    string        `json:"last_error,omitempty"`
	TotalDelay   time.Duration `json:"total_delay"`
}

// StreamConfiguration provides configuration validation and defaults
type StreamConfiguration interface {
	// ValidateStreamConfig validates a stream configuration
	ValidateStreamConfig(config config.StreamConfig) error
	
	// ApplyDefaults applies default values to stream configuration
	ApplyDefaults(config *config.StreamConfig) error
	
	// MergeConfigs merges two stream configurations
	MergeConfigs(base, override config.StreamConfig) config.StreamConfig
	
	// GetConfigSchema returns the JSON schema for stream configuration
	GetConfigSchema(streamType string) (string, error)
}

// StreamFilter filters events based on configured criteria
type StreamFilter interface {
	// ShouldInclude determines if an event should be included
	ShouldInclude(event models.ChangeEvent, config config.StreamConfig) (bool, error)
	
	// ValidateFilters validates filter configuration
	ValidateFilters(filters map[string]interface{}) error
}

// StreamDiscovery discovers available streams in a data source
type StreamDiscovery interface {
	// DiscoverStreams discovers available streams in a data source
	DiscoverStreams(ctx context.Context, config config.SourceConfig) ([]StreamInfo, error)
	
	// GetStreamSchema returns schema information for a stream
	GetStreamSchema(ctx context.Context, config config.SourceConfig, streamName string) (*StreamSchema, error)
}

// StreamInfo contains information about a discovered stream
type StreamInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Database    string            `json:"database,omitempty"`
	Collection  string            `json:"collection,omitempty"`
	Table       string            `json:"table,omitempty"`
	Schema      string            `json:"schema,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// StreamSchema contains schema information for a stream
type StreamSchema struct {
	Fields      []FieldInfo       `json:"fields"`
	Indexes     []IndexInfo       `json:"indexes,omitempty"`
	Constraints []ConstraintInfo  `json:"constraints,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// FieldInfo contains information about a field in a stream
type FieldInfo struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Nullable    bool        `json:"nullable"`
	PrimaryKey  bool        `json:"primary_key,omitempty"`
	DefaultValue interface{} `json:"default_value,omitempty"`
	Description string      `json:"description,omitempty"`
}

// IndexInfo contains information about an index
type IndexInfo struct {
	Name    string   `json:"name"`
	Fields  []string `json:"fields"`
	Unique  bool     `json:"unique"`
	Type    string   `json:"type,omitempty"`
}

// ConstraintInfo contains information about a constraint
type ConstraintInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // PRIMARY_KEY, FOREIGN_KEY, UNIQUE, CHECK
	Fields      []string `json:"fields"`
	RefTable    string   `json:"ref_table,omitempty"`
	RefFields   []string `json:"ref_fields,omitempty"`
	Definition  string   `json:"definition,omitempty"`
}

// DefaultStreamFactory provides a default implementation of StreamFactory
type DefaultStreamFactory struct {
	streamType string
	createFunc func(config config.StreamConfig) (models.Stream, error)
}

// NewDefaultStreamFactory creates a new default stream factory
func NewDefaultStreamFactory(streamType string, createFunc func(config config.StreamConfig) (models.Stream, error)) *DefaultStreamFactory {
	return &DefaultStreamFactory{
		streamType: streamType,
		createFunc: createFunc,
	}
}

// CreateStream implements StreamFactory
func (f *DefaultStreamFactory) CreateStream(config config.StreamConfig) (models.Stream, error) {
	return f.createFunc(config)
}

// GetSupportedTypes implements StreamFactory
func (f *DefaultStreamFactory) GetSupportedTypes() []string {
	return []string{f.streamType}
}

// ValidateConfig implements StreamFactory
func (f *DefaultStreamFactory) ValidateConfig(config config.StreamConfig) error {
	// Basic validation - in a real implementation, this would be more comprehensive
	if config.Name == "" {
		return fmt.Errorf("stream name is required")
	}
	return nil
}