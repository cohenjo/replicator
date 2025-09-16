package models

import (
	"context"
	"errors"
	"time"
	
	"github.com/cohenjo/replicator/pkg/config"
)

// Common error definitions
var (
	ErrStreamNotFound       = errors.New("stream not found")
	ErrStreamAlreadyExists  = errors.New("stream already exists")
	ErrStreamNotRunning     = errors.New("stream is not running")
	ErrStreamAlreadyRunning = errors.New("stream is already running")
	ErrInvalidConfiguration = errors.New("invalid configuration")
	ErrConnectionFailed     = errors.New("connection failed")
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrTransformationFailed = errors.New("transformation failed")
	ErrCheckpointFailed     = errors.New("checkpoint operation failed")
)

// ReplicationError represents a detailed replication error
type ReplicationError struct {
	StreamName    string                 `json:"stream_name"`
	ErrorType     string                 `json:"error_type"`
	Message       string                 `json:"message"`
	Timestamp     time.Time              `json:"timestamp"`
	EventID       string                 `json:"event_id,omitempty"`
	RetryCount    int                    `json:"retry_count"`
	Fatal         bool                   `json:"fatal"`
	Context       map[string]interface{} `json:"context,omitempty"`
	OriginalError error                  `json:"-"`
}

// Error implements the error interface
func (e *ReplicationError) Error() string {
	return e.Message
}

// Unwrap returns the original error for error unwrapping
func (e *ReplicationError) Unwrap() error {
	return e.OriginalError
}

// ConnectionInfo represents connection information for a data source or target
type ConnectionInfo struct {
	Type        string    `json:"type"`
	Endpoint    string    `json:"endpoint"`
	Database    string    `json:"database,omitempty"`
	Connected   bool      `json:"connected"`
	LastAttempt time.Time `json:"last_attempt"`
	ErrorCount  int       `json:"error_count"`
	LastError   string    `json:"last_error,omitempty"`
}

// EventStatistics represents statistics for event processing
type EventStatistics struct {
	StreamName         string        `json:"stream_name"`
	TotalEvents        int64         `json:"total_events"`
	InsertEvents       int64         `json:"insert_events"`
	UpdateEvents       int64         `json:"update_events"`
	DeleteEvents       int64         `json:"delete_events"`
	ReplaceEvents      int64         `json:"replace_events"`
	ErrorEvents        int64         `json:"error_events"`
	TransformedEvents  int64         `json:"transformed_events"`
	SkippedEvents      int64         `json:"skipped_events"`
	AverageEventSize   float64       `json:"average_event_size_bytes"`
	ProcessingTime     time.Duration `json:"processing_time"`
	AverageLatency     time.Duration `json:"average_latency"`
	WindowStart        time.Time     `json:"window_start"`
	WindowEnd          time.Time     `json:"window_end"`
}

// StreamPerformance represents performance metrics for a stream
type StreamPerformance struct {
	StreamName           string        `json:"stream_name"`
	Throughput           float64       `json:"throughput_events_per_second"`
	BytesPerSecond       float64       `json:"bytes_per_second"`
	AverageLatency       time.Duration `json:"average_latency"`
	P95Latency           time.Duration `json:"p95_latency"`
	P99Latency           time.Duration `json:"p99_latency"`
	ErrorRate            float64       `json:"error_rate"`
	RetryRate            float64       `json:"retry_rate"`
	BackpressureEvents   int64         `json:"backpressure_events"`
	QueueDepth           int           `json:"queue_depth"`
	MemoryUsage          int64         `json:"memory_usage_bytes"`
	CPUUsage             float64       `json:"cpu_usage_percent"`
	NetworkBytesReceived int64         `json:"network_bytes_received"`
	NetworkBytesSent     int64         `json:"network_bytes_sent"`
}

// AggregatedMetrics represents system-wide aggregated metrics
type AggregatedMetrics struct {
	Timestamp            time.Time              `json:"timestamp"`
	TotalStreams         int                    `json:"total_streams"`
	ActiveStreams        int                    `json:"active_streams"`
	TotalEventsProcessed int64                  `json:"total_events_processed"`
	TotalBytesProcessed  int64                  `json:"total_bytes_processed"`
	AverageThroughput    float64                `json:"average_throughput_eps"`
	OverallErrorRate     float64                `json:"overall_error_rate"`
	SystemCPUUsage       float64                `json:"system_cpu_usage_percent"`
	SystemMemoryUsage    int64                  `json:"system_memory_usage_bytes"`
	UptimeSeconds        float64                `json:"uptime_seconds"`
	StreamMetrics        []StreamPerformance    `json:"stream_metrics"`
	Connections          []ConnectionInfo       `json:"connections"`
}

// RetryPolicy represents retry configuration for operations
type RetryPolicy struct {
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor"`
	RetryableErrors []string      `json:"retryable_errors"`
}

// CircuitBreakerConfig represents circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled"`
	FailureThreshold int           `json:"failure_threshold"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout"`
	HalfOpenRequests int           `json:"half_open_requests"`
}

// QualityOfService represents QoS settings for a stream
type QualityOfService struct {
	DeliveryGuarantee string               `json:"delivery_guarantee"` // at_least_once, at_most_once, exactly_once
	MaxLatency        time.Duration        `json:"max_latency"`
	RetryPolicy       RetryPolicy          `json:"retry_policy"`
	CircuitBreaker    CircuitBreakerConfig `json:"circuit_breaker"`
	RateLimiting      *RateLimitConfig     `json:"rate_limiting,omitempty"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled     bool    `json:"enabled"`
	EventsPerSecond float64 `json:"events_per_second"`
	BurstSize   int     `json:"burst_size"`
}

// MonitoringConfig represents monitoring and alerting configuration
type MonitoringConfig struct {
	Enabled         bool                   `json:"enabled"`
	HealthChecks    []HealthCheckConfig    `json:"health_checks"`
	Alerts          []AlertConfig          `json:"alerts"`
	MetricRetention time.Duration          `json:"metric_retention"`
	ExportInterval  time.Duration          `json:"export_interval"`
}

// HealthCheckConfig represents configuration for a health check
type HealthCheckConfig struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"` // connection, latency, throughput, error_rate
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

// AlertConfig represents configuration for an alert
type AlertConfig struct {
	Name        string                 `json:"name"`
	Condition   string                 `json:"condition"` // threshold, anomaly, pattern
	Metric      string                 `json:"metric"`
	Threshold   float64                `json:"threshold,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Actions     []string               `json:"actions"` // log, webhook, email
	Params      map[string]interface{} `json:"params,omitempty"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	TLS             config.TLSConfig       `json:"tls"`
	Authentication  AuthConfig      `json:"authentication"`
	Authorization   AuthzConfig     `json:"authorization"`
	Encryption      EncryptionConfig `json:"encryption"`
	AuditLogging    bool            `json:"audit_logging"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled  bool              `json:"enabled"`
	Provider string            `json:"provider"` // azure_ad, oauth2, api_key
	Options  map[string]interface{} `json:"options,omitempty"`
}

// AuthzConfig represents authorization configuration
type AuthzConfig struct {
	Enabled bool              `json:"enabled"`
	Model   string            `json:"model"` // rbac, abac
	Rules   []AuthzRule       `json:"rules"`
}

// AuthzRule represents an authorization rule
type AuthzRule struct {
	Subject   string   `json:"subject"`
	Resource  string   `json:"resource"`
	Actions   []string `json:"actions"`
	Condition string   `json:"condition,omitempty"`
}

// EncryptionConfig represents encryption configuration
type EncryptionConfig struct {
	InTransit EncryptionSettings `json:"in_transit"`
	AtRest    EncryptionSettings `json:"at_rest"`
}

// EncryptionSettings represents encryption settings
type EncryptionSettings struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"`
	KeySource string `json:"key_source"` // local, azure_key_vault, aws_kms
	KeyID     string `json:"key_id,omitempty"`
}

// EventFilter represents filtering configuration for events
type EventFilter struct {
	IncludeOperations []string               `json:"include_operations,omitempty"`
	ExcludeOperations []string               `json:"exclude_operations,omitempty"`
	IncludeDatabases  []string               `json:"include_databases,omitempty"`
	ExcludeDatabases  []string               `json:"exclude_databases,omitempty"`
	IncludeCollections []string              `json:"include_collections,omitempty"`
	ExcludeCollections []string              `json:"exclude_collections,omitempty"`
	FieldFilters      []FieldFilter          `json:"field_filters,omitempty"`
	CustomFilter      string                 `json:"custom_filter,omitempty"` // JavaScript expression
}

// FieldFilter represents a field-level filter
type FieldFilter struct {
	Field     string      `json:"field"`
	Operator  string      `json:"operator"` // eq, ne, gt, lt, gte, lte, in, nin, regex
	Value     interface{} `json:"value"`
	Include   bool        `json:"include"` // true to include, false to exclude
}

// StreamContext represents execution context for a stream
type StreamContext struct {
	Context     context.Context
	StreamName  string
	StartTime   time.Time
	Metrics     *ReplicationMetrics
	Logger      interface{} // Logger interface
	Tracer      interface{} // Tracer interface
	Checkpointer interface{} // Checkpointer interface
}

// NewStreamContext creates a new stream context
func NewStreamContext(ctx context.Context, streamName string) *StreamContext {
	return &StreamContext{
		Context:    ctx,
		StreamName: streamName,
		StartTime:  time.Now(),
		Metrics:    &ReplicationMetrics{StreamName: streamName},
	}
}

// WithTimeout creates a new context with timeout
func (sc *StreamContext) WithTimeout(timeout time.Duration) (*StreamContext, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(sc.Context, timeout)
	newContext := *sc
	newContext.Context = ctx
	return &newContext, cancel
}

// WithCancel creates a new context with cancel
func (sc *StreamContext) WithCancel() (*StreamContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(sc.Context)
	newContext := *sc
	newContext.Context = ctx
	return &newContext, cancel
}