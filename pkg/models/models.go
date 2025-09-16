package models

import (
	"context"
	"time"
	
	"github.com/cohenjo/replicator/pkg/config"
)

// StreamState represents the runtime state of a replication stream
type StreamState struct {
	Name               string                 `json:"name"`
	Status             config.StreamStatus    `json:"status"`
	LastProcessedTime  *time.Time             `json:"last_processed_time,omitempty"`
	EventsProcessed    int64                  `json:"events_processed"`
	EventsPerSecond    float64                `json:"events_per_second"`
	LastError          *string                `json:"last_error,omitempty"`
	ErrorCount         int64                  `json:"error_count"`
	ResumeToken        string                 `json:"resume_token,omitempty"`
	Checkpoint         map[string]interface{} `json:"checkpoint,omitempty"`
	StartedAt          *time.Time             `json:"started_at,omitempty"`
	StoppedAt          *time.Time             `json:"stopped_at,omitempty"`
	LastHeartbeatAt    *time.Time             `json:"last_heartbeat_at,omitempty"`
}

// ChangeEvent represents a change event from a data source
type ChangeEvent struct {
	ID            string                 `json:"id"`
	StreamName    string                 `json:"stream_name"`
	OperationType string                 `json:"operation_type"` // insert, update, delete, replace
	Timestamp     time.Time              `json:"timestamp"`
	Database      string                 `json:"database,omitempty"`
	Collection    string                 `json:"collection,omitempty"`
	Table         string                 `json:"table,omitempty"`
	DocumentKey   map[string]interface{} `json:"document_key,omitempty"`
	FullDocument  map[string]interface{} `json:"full_document,omitempty"`
	UpdateFields  map[string]interface{} `json:"update_fields,omitempty"`
	ResumeToken   string                 `json:"resume_token,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ReplicationMetrics represents metrics for replication monitoring
type ReplicationMetrics struct {
	StreamName          string    `json:"stream_name"`
	EventsProcessed     int64     `json:"events_processed"`
	EventsPerSecond     float64   `json:"events_per_second"`
	BytesProcessed      int64     `json:"bytes_processed"`
	BytesPerSecond      float64   `json:"bytes_per_second"`
	ErrorCount          int64     `json:"error_count"`
	ErrorRate           float64   `json:"error_rate"`
	ReplicationLag      float64   `json:"replication_lag_seconds"`
	LastProcessedTime   time.Time `json:"last_processed_time"`
	LastHeartbeatTime   time.Time `json:"last_heartbeat_time"`
	UpstreamConnected   bool      `json:"upstream_connected"`
	DownstreamConnected bool      `json:"downstream_connected"`
}

// HealthStatus represents the overall health status
type HealthStatus struct {
	Status      string               `json:"status"` // healthy, degraded, unhealthy
	Timestamp   time.Time            `json:"timestamp"`
	Uptime      time.Duration        `json:"uptime"`
	Version     string               `json:"version"`
	StreamCount int                  `json:"stream_count"`
	Streams     map[string]StreamState `json:"streams"`
	Checks      map[string]CheckResult `json:"checks"`
}

// CheckResult represents the result of a health check
type CheckResult struct {
	Status    string     `json:"status"` // pass, fail, warn
	Message   string     `json:"message,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
	Duration  *time.Duration `json:"duration,omitempty"`
}

// Stream represents a replication stream interface
type Stream interface {
	// Start begins the replication stream
	Start(ctx context.Context) error
	
	// Stop gracefully stops the replication stream
	Stop(ctx context.Context) error
	
	// Pause temporarily pauses the replication stream
	Pause(ctx context.Context) error
	
	// Resume resumes a paused replication stream
	Resume(ctx context.Context) error
	
	// GetState returns the current state of the stream
	GetState() StreamState
	
	// GetConfig returns the configuration of the stream
	GetConfig() config.StreamConfig
	
	// GetMetrics returns current metrics for the stream
	GetMetrics() ReplicationMetrics
	
	// SetCheckpoint updates the stream checkpoint
	SetCheckpoint(checkpoint map[string]interface{}) error
	
	// GetCheckpoint returns the current checkpoint
	GetCheckpoint() (map[string]interface{}, error)
}

// StreamManager represents a manager for multiple replication streams
type StreamManager interface {
	// CreateStream creates a new replication stream
	CreateStream(config config.StreamConfig) (Stream, error)
	
	// GetStream retrieves a stream by name
	GetStream(name string) (Stream, bool)
	
	// ListStreams returns all configured streams
	ListStreams() []Stream
	
	// DeleteStream removes a stream
	DeleteStream(name string) error
	
	// StartAll starts all configured streams
	StartAll(ctx context.Context) error
	
	// StopAll stops all running streams
	StopAll(ctx context.Context) error
	
	// GetHealthStatus returns overall health status
	GetHealthStatus() HealthStatus
}

// Transformer represents a data transformation interface
type Transformer interface {
	// Transform applies transformation to a change event
	Transform(event ChangeEvent) (ChangeEvent, error)
	
	// ValidateSpec validates the transformation specification
	ValidateSpec(spec string) error
}

// EventProcessor represents an event processing interface
type EventProcessor interface {
	// ProcessEvent processes a single change event
	ProcessEvent(ctx context.Context, event ChangeEvent) error
	
	// ProcessBatch processes a batch of change events
	ProcessBatch(ctx context.Context, events []ChangeEvent) error
	
	// GetMetrics returns processing metrics
	GetMetrics() ReplicationMetrics
}