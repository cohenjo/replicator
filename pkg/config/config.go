package config

import (
	"context"
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// StreamStatus represents the current status of a replication stream
type StreamStatus string

const (
	StreamStatusStopped  StreamStatus = "stopped"
	StreamStatusStarting StreamStatus = "starting"
	StreamStatusRunning  StreamStatus = "running"
	StreamStatusPaused   StreamStatus = "paused"
	StreamStatusError    StreamStatus = "error"
	StreamStatusStopping StreamStatus = "stopping"
)

// SourceType represents the type of data source
type SourceType string

const (
	SourceTypeMongoDB     SourceType = "mongodb"
	SourceTypeMySQL       SourceType = "mysql"
	SourceTypePostgreSQL  SourceType = "postgresql"
	SourceTypeCosmosDB    SourceType = "cosmosdb"
	SourceTypeKafka       SourceType = "kafka"
)

// TargetType represents the type of data target
type TargetType string

const (
	TargetTypeMongoDB     TargetType = "mongodb"
	TargetTypeMySQL       TargetType = "mysql"
	TargetTypePostgreSQL  TargetType = "postgresql"
	TargetTypeCosmosDB    TargetType = "cosmosdb"
	TargetTypeKafka       TargetType = "kafka"
	TargetTypeElastic     TargetType = "elasticsearch"
)

// TransformationType represents the type of data transformation
type TransformationType string

const (
	TransformationTypeKazaam TransformationType = "kazaam"
	TransformationTypeNone   TransformationType = "none"
)

// TransformationConfig represents configuration for a transformation engine
type TransformationConfig struct {
	Type TransformationType `json:"type" yaml:"type"`
}

// SourceConfig represents configuration for a data source
type SourceConfig struct {
	Type     SourceType             `json:"type" yaml:"type"`
	URI      string                 `json:"uri,omitempty" yaml:"uri,omitempty"`
	Host     string                 `json:"host,omitempty" yaml:"host,omitempty"`
	Port     int                    `json:"port,omitempty" yaml:"port,omitempty"`
	Database string                 `json:"database,omitempty" yaml:"database,omitempty"`
	Username string                 `json:"username,omitempty" yaml:"username,omitempty"`
	Password string                 `json:"password,omitempty" yaml:"password,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty" yaml:"options,omitempty"`
}

// TargetConfig represents configuration for a data target
type TargetConfig struct {
	Type     TargetType             `json:"type" yaml:"type"`
	URI      string                 `json:"uri,omitempty" yaml:"uri,omitempty"`
	Host     string                 `json:"host,omitempty" yaml:"host,omitempty"`
	Port     int                    `json:"port,omitempty" yaml:"port,omitempty"`
	Database string                 `json:"database,omitempty" yaml:"database,omitempty"`
	Username string                 `json:"username,omitempty" yaml:"username,omitempty"`
	Password string                 `json:"password,omitempty" yaml:"password,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty" yaml:"options,omitempty"`
}

// LegacyTransformationConfig represents legacy configuration for data transformation (deprecated)
type LegacyTransformationConfig struct {
	Type TransformationType `json:"type" yaml:"type"`
	Spec string             `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// StreamConfig represents the complete configuration for a replication stream
type StreamConfig struct {
	Name           string                       `json:"name" yaml:"name"`
	Source         SourceConfig                 `json:"source" yaml:"source"`
	Target         TargetConfig                 `json:"target" yaml:"target"`
	Transformation *TransformationRulesConfig  `json:"transformation,omitempty" yaml:"transformation,omitempty"`
	BatchSize      int                          `json:"batch_size,omitempty" yaml:"batch_size,omitempty"`
	BufferSize     int                          `json:"buffer_size,omitempty" yaml:"buffer_size,omitempty"`
	Enabled        bool                         `json:"enabled" yaml:"enabled"`
	
	// Legacy field for backwards compatibility
	LegacyTransformation *LegacyTransformationConfig `json:"legacy_transformation,omitempty" yaml:"legacy_transformation,omitempty"`
}

// TransformationRulesConfig represents the configuration for stream-specific transformation rules
type TransformationRulesConfig struct {
	Enabled       bool                      `json:"enabled" yaml:"enabled"`
	Engine        string                    `json:"engine,omitempty" yaml:"engine,omitempty"` // "kazaam", "jq", "lua", "javascript"
	Rules         []TransformationRule      `json:"rules" yaml:"rules"`
	ErrorHandling ErrorHandlingPolicy       `json:"error_handling,omitempty" yaml:"error_handling,omitempty"`
	Metrics       TransformationMetricsConfig `json:"metrics,omitempty" yaml:"metrics,omitempty"`
}

// TransformationRule represents a transformation rule configuration
type TransformationRule struct {
	Name         string                 `json:"name" yaml:"name"`
	Description  string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Enabled      bool                   `json:"enabled" yaml:"enabled"`
	Priority     int                    `json:"priority" yaml:"priority"` // Lower number = higher priority
	Conditions   []Condition            `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Actions      []Action               `json:"actions" yaml:"actions"`
	ErrorHandling ErrorHandlingPolicy   `json:"error_handling,omitempty" yaml:"error_handling,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Condition represents a condition for applying a transformation
type Condition struct {
	Field    string      `json:"field" yaml:"field"`         // JSONPath or field name
	Operator string      `json:"operator" yaml:"operator"`   // eq, ne, gt, lt, contains, exists, etc.
	Value    interface{} `json:"value" yaml:"value"`         // Value to compare against
	Type     string      `json:"type,omitempty" yaml:"type,omitempty"` // string, number, boolean, null
}

// Action represents a transformation action
type Action struct {
	Type     string                 `json:"type" yaml:"type"`         // "kazaam", "jq", "lua", "javascript"
	Spec     string                 `json:"spec" yaml:"spec"`         // Transformation specification
	Target   string                 `json:"target,omitempty" yaml:"target,omitempty"` // Target field for output
	Config   map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"` // Action-specific config
}

// ErrorHandlingPolicy defines how errors should be handled during transformation
type ErrorHandlingPolicy struct {
	Strategy        string        `json:"strategy" yaml:"strategy"`                 // fail_fast, skip, retry, dead_letter
	MaxRetries      int           `json:"max_retries,omitempty" yaml:"max_retries,omitempty"`
	RetryDelay      string        `json:"retry_delay,omitempty" yaml:"retry_delay,omitempty"` // Duration string
	DeadLetterTopic string        `json:"dead_letter_topic,omitempty" yaml:"dead_letter_topic,omitempty"`
	LogErrors       bool          `json:"log_errors" yaml:"log_errors"`
	Metrics         bool          `json:"metrics" yaml:"metrics"`
}

// TransformationMetricsConfig represents configuration for transformation metrics
type TransformationMetricsConfig struct {
	Enabled           bool   `json:"enabled" yaml:"enabled"`
	CollectionInterval string `json:"collection_interval,omitempty" yaml:"collection_interval,omitempty"` // Duration string
	DetailedMetrics   bool   `json:"detailed_metrics,omitempty" yaml:"detailed_metrics,omitempty"`
}

// Validate validates the stream configuration
func (s *StreamConfig) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("stream name cannot be empty")
	}
	
	if s.Source.Type == "" {
		return fmt.Errorf("source type cannot be empty")
	}
	
	if s.Target.Type == "" {
		return fmt.Errorf("target type cannot be empty")
	}
	
	validSourceTypes := map[SourceType]bool{
		SourceTypeMongoDB: true, SourceTypeMySQL: true, SourceTypePostgreSQL: true,
		SourceTypeCosmosDB: true, SourceTypeKafka: true,
	}
	if !validSourceTypes[s.Source.Type] {
		return fmt.Errorf("invalid source type: %s", s.Source.Type)
	}
	
	validTargetTypes := map[TargetType]bool{
		TargetTypeMongoDB: true, TargetTypeMySQL: true, TargetTypePostgreSQL: true,
		TargetTypeCosmosDB: true, TargetTypeKafka: true, TargetTypeElastic: true,
	}
	if !validTargetTypes[s.Target.Type] {
		return fmt.Errorf("invalid target type: %s", s.Target.Type)
	}
	
	return nil
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Host            string        `json:"host" yaml:"host"`
	Port            int           `json:"port" yaml:"port"`
	ReadTimeout     time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout" yaml:"write_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout"`
	TLS             *TLSConfig    `json:"tls,omitempty" yaml:"tls,omitempty"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	CertFile string `json:"cert_file" yaml:"cert_file"`
	KeyFile  string `json:"key_file" yaml:"key_file"`
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	Enabled        bool          `json:"enabled" yaml:"enabled"`
	Port           int           `json:"port" yaml:"port"`
	Path           string        `json:"path" yaml:"path"`
	Interval       time.Duration `json:"interval" yaml:"interval"`
	Namespace      string        `json:"namespace" yaml:"namespace"`
	AzureMonitor   bool          `json:"azure_monitor" yaml:"azure_monitor"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level    string `json:"level" yaml:"level"`     // debug, info, warn, error
	Format   string `json:"format" yaml:"format"`   // json, text
	Output   string `json:"output" yaml:"output"`   // stdout, stderr, file
	File     string `json:"file,omitempty" yaml:"file,omitempty"`
	Rotation bool   `json:"rotation" yaml:"rotation"`
}

// AzureConfig represents Azure-specific configuration
type AzureConfig struct {
	Authentication AuthenticationConfig `json:"authentication" yaml:"authentication"`
	KeyVault       KeyVaultConfig       `json:"key_vault" yaml:"key_vault"`
	CosmosDB       CosmosDBConfig       `json:"cosmos_db" yaml:"cosmos_db"`
	Monitor        MonitorConfig        `json:"monitor" yaml:"monitor"`
}

// AuthenticationConfig represents Azure authentication configuration
type AuthenticationConfig struct {
	Method          string `json:"method" yaml:"method"`                     // service_principal, managed_identity, cli
	TenantID        string `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	ClientID        string `json:"client_id,omitempty" yaml:"client_id,omitempty"`
	ClientSecret    string `json:"client_secret,omitempty" yaml:"client_secret,omitempty"`
	CertificatePath string `json:"certificate_path,omitempty" yaml:"certificate_path,omitempty"`
}

// KeyVaultConfig represents Azure Key Vault configuration
type KeyVaultConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	VaultURL  string `json:"vault_url,omitempty" yaml:"vault_url,omitempty"`
	SecretPrefix string `json:"secret_prefix,omitempty" yaml:"secret_prefix,omitempty"`
}

// CosmosDBConfig represents Cosmos DB specific configuration
type CosmosDBConfig struct {
	DefaultConsistencyLevel string `json:"default_consistency_level" yaml:"default_consistency_level"`
	MaxRetryAttemptsOnThrottledRequests int `json:"max_retry_attempts_on_throttled_requests" yaml:"max_retry_attempts_on_throttled_requests"`
	MaxRetryWaitTimeInSeconds int `json:"max_retry_wait_time_in_seconds" yaml:"max_retry_wait_time_in_seconds"`
	PreferredRegions []string `json:"preferred_regions,omitempty" yaml:"preferred_regions,omitempty"`
}

// MonitorConfig represents Azure Monitor configuration
type MonitorConfig struct {
	Enabled               bool   `json:"enabled" yaml:"enabled"`
	WorkspaceID          string `json:"workspace_id,omitempty" yaml:"workspace_id,omitempty"`
	InstrumentationKey   string `json:"instrumentation_key,omitempty" yaml:"instrumentation_key,omitempty"`
	ConnectionString     string `json:"connection_string,omitempty" yaml:"connection_string,omitempty"`
}

// OpenTelemetryConfig represents OpenTelemetry configuration
type OpenTelemetryConfig struct {
	Enabled     bool            `json:"enabled" yaml:"enabled"`
	ServiceName string          `json:"service_name" yaml:"service_name"`
	Tracing     TracingConfig   `json:"tracing" yaml:"tracing"`
	Metrics     OTelMetricsConfig `json:"metrics" yaml:"metrics"`
}

// TracingConfig represents OpenTelemetry tracing configuration
type TracingConfig struct {
	Enabled    bool    `json:"enabled" yaml:"enabled"`
	Endpoint   string  `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	SampleRate float64 `json:"sample_rate" yaml:"sample_rate"`
}

// OTelMetricsConfig represents OpenTelemetry metrics configuration  
type OTelMetricsConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Interval time.Duration `json:"interval" yaml:"interval"`
}

// TelemetryConfig represents telemetry configuration
type TelemetryConfig struct {
	Enabled         bool              `json:"enabled" yaml:"enabled"`
	ServiceName     string            `json:"service_name" yaml:"service_name"`
	ServiceVersion  string            `json:"service_version" yaml:"service_version"`
	Environment     string            `json:"environment" yaml:"environment"`
	MetricsEnabled  bool              `json:"metrics_enabled" yaml:"metrics_enabled"`
	TracingEnabled  bool              `json:"tracing_enabled" yaml:"tracing_enabled"`
	OTLPEndpoint    string            `json:"otlp_endpoint" yaml:"otlp_endpoint"`
	MetricsInterval time.Duration     `json:"metrics_interval" yaml:"metrics_interval"`
	Labels          map[string]string `json:"labels" yaml:"labels"`
}

// Config represents the main application configuration
type Config struct {
	Server      ServerConfig      `json:"server" yaml:"server"`
	Streams     []StreamConfig    `json:"streams" yaml:"streams"`
	Metrics     MetricsConfig     `json:"metrics" yaml:"metrics"`
	Logging     LoggingConfig     `json:"logging" yaml:"logging"`
	Azure       AzureConfig       `json:"azure" yaml:"azure"`
	OpenTelemetry OpenTelemetryConfig `json:"opentelemetry" yaml:"opentelemetry"`
	Telemetry   TelemetryConfig   `json:"telemetry" yaml:"telemetry"`
	
	// Legacy fields for backwards compatibility
	Debug              bool                   `json:"debug,omitempty" yaml:"debug,omitempty"`
	Execute            bool                   `json:"execute,omitempty" yaml:"execute,omitempty"`
	StreamQueueLength  int                    `json:"stream_queue_length,omitempty" yaml:"stream_queue_length,omitempty"`
	EstuaryQueueLength int                    `json:"estuary_queue_length,omitempty" yaml:"estuary_queue_length,omitempty"`
	DBUser             string                 `json:"db_user,omitempty" yaml:"db_user,omitempty"`
	DBPasswd           string                 `json:"db_passwd,omitempty" yaml:"db_passwd,omitempty"`
	MyDBUser           string                 `json:"my_db_user,omitempty" yaml:"my_db_user,omitempty"`
	MyDBPasswd         string                 `json:"my_db_passwd,omitempty" yaml:"my_db_passwd,omitempty"`
	WaterFlowsConfig   *WaterFlowsConfig      `json:"water_flows_config,omitempty" yaml:"water_flows_config,omitempty"`
	LegacyStreams      []WaterFlowsConfig     `json:"legacy_streams,omitempty" yaml:"legacy_streams,omitempty"`
	Estuaries          []WaterFlowsConfig     `json:"estuaries,omitempty" yaml:"estuaries,omitempty"`
	Transforms         []TransformOperation   `json:"transforms,omitempty" yaml:"transforms,omitempty"`
}

// Legacy types for backwards compatibility
type WaterFlowsConfig struct {
	Type       string `json:"type" yaml:"type"`
	Host       string `json:"host" yaml:"host"`
	Port       int    `json:"port" yaml:"port"`
	Schema     string `json:"schema" yaml:"schema"`
	Collection string `json:"collection" yaml:"collection"`
	
// MongoDB specific fields
MongoURI                     string   `json:"mongo_uri,omitempty" yaml:"mongo_uri,omitempty"`
MongoDatabaseName            string   `json:"mongo_database_name,omitempty" yaml:"mongo_database_name,omitempty"`
MongoCollectionName          string   `json:"mongo_collection_name,omitempty" yaml:"mongo_collection_name,omitempty"`
MongoStartAtOperationTime    bool     `json:"mongo_start_at_operation_time,omitempty" yaml:"mongo_start_at_operation_time,omitempty"`
MongoFullDocument            string   `json:"mongo_full_document,omitempty" yaml:"mongo_full_document,omitempty"`
MongoResumeAfter             string   `json:"mongo_resume_after,omitempty" yaml:"mongo_resume_after,omitempty"`
MongoMaxAwaitTime            int      `json:"mongo_max_await_time,omitempty" yaml:"mongo_max_await_time,omitempty"`
MongoBatchSize               int32    `json:"mongo_batch_size,omitempty" yaml:"mongo_batch_size,omitempty"`
MongoIncludeOperations       []string `json:"mongo_include_operations,omitempty" yaml:"mongo_include_operations,omitempty"`
MongoExcludeOperations       []string `json:"mongo_exclude_operations,omitempty" yaml:"mongo_exclude_operations,omitempty"`

// MongoDB authentication specific fields
MongoAuthMethod              string   `json:"mongo_auth_method,omitempty" yaml:"mongo_auth_method,omitempty"`
MongoTenantID                string   `json:"mongo_tenant_id,omitempty" yaml:"mongo_tenant_id,omitempty"`
MongoClientID                string   `json:"mongo_client_id,omitempty" yaml:"mongo_client_id,omitempty"`
MongoScopes                  []string `json:"mongo_scopes,omitempty" yaml:"mongo_scopes,omitempty"`
MongoRefreshBeforeExpiry     string   `json:"mongo_refresh_before_expiry,omitempty" yaml:"mongo_refresh_before_expiry,omitempty"`
	
	// Cosmos DB specific fields
	CosmosEndpoint               string   `json:"cosmos_endpoint,omitempty" yaml:"cosmos_endpoint,omitempty"`
	CosmosDatabaseName           string   `json:"cosmos_database_name,omitempty" yaml:"cosmos_database_name,omitempty"`
	CosmosContainerName          string   `json:"cosmos_container_name,omitempty" yaml:"cosmos_container_name,omitempty"`
	CosmosStartFromBeginning     bool     `json:"cosmos_start_from_beginning,omitempty" yaml:"cosmos_start_from_beginning,omitempty"`
	CosmosMaxItemCount           int32    `json:"cosmos_max_item_count,omitempty" yaml:"cosmos_max_item_count,omitempty"`
	CosmosPollInterval           int      `json:"cosmos_poll_interval,omitempty" yaml:"cosmos_poll_interval,omitempty"`
	CosmosIncludeOperations      []string `json:"cosmos_include_operations,omitempty" yaml:"cosmos_include_operations,omitempty"`
	CosmosExcludeOperations      []string `json:"cosmos_exclude_operations,omitempty" yaml:"cosmos_exclude_operations,omitempty"`
	
	// MySQL specific fields
	MySQLServerID                uint32   `json:"mysql_server_id,omitempty" yaml:"mysql_server_id,omitempty"`
	MySQLBinlogFormat            string   `json:"mysql_binlog_format,omitempty" yaml:"mysql_binlog_format,omitempty"`
	MySQLBinlogPosition          string   `json:"mysql_binlog_position,omitempty" yaml:"mysql_binlog_position,omitempty"`
	MySQLBinlogFile              string   `json:"mysql_binlog_file,omitempty" yaml:"mysql_binlog_file,omitempty"`
	MySQLHeartbeatPeriod         int      `json:"mysql_heartbeat_period,omitempty" yaml:"mysql_heartbeat_period,omitempty"`  // seconds
	MySQLReadTimeout             int      `json:"mysql_read_timeout,omitempty" yaml:"mysql_read_timeout,omitempty"`        // seconds
	MySQLIncludeTables           []string `json:"mysql_include_tables,omitempty" yaml:"mysql_include_tables,omitempty"`
	MySQLExcludeTables           []string `json:"mysql_exclude_tables,omitempty" yaml:"mysql_exclude_tables,omitempty"`
	MySQLIncludeOperations       []string `json:"mysql_include_operations,omitempty" yaml:"mysql_include_operations,omitempty"`
	MySQLExcludeOperations       []string `json:"mysql_exclude_operations,omitempty" yaml:"mysql_exclude_operations,omitempty"`
	MySQLMaxRetries              int      `json:"mysql_max_retries,omitempty" yaml:"mysql_max_retries,omitempty"`
	MySQLRetryDelay              int      `json:"mysql_retry_delay,omitempty" yaml:"mysql_retry_delay,omitempty"`           // seconds
	MySQLMaxBackoff              int      `json:"mysql_max_backoff,omitempty" yaml:"mysql_max_backoff,omitempty"`          // seconds
	MySQLConnTimeout             int      `json:"mysql_conn_timeout,omitempty" yaml:"mysql_conn_timeout,omitempty"`        // seconds
	MySQLUseSSL                  bool     `json:"mysql_use_ssl,omitempty" yaml:"mysql_use_ssl,omitempty"`
	MySQLSSLCert                 string   `json:"mysql_ssl_cert,omitempty" yaml:"mysql_ssl_cert,omitempty"`
	MySQLSSLKey                  string   `json:"mysql_ssl_key,omitempty" yaml:"mysql_ssl_key,omitempty"`
	MySQLSSLCa                   string   `json:"mysql_ssl_ca,omitempty" yaml:"mysql_ssl_ca,omitempty"`
	MySQLSSLMode                 string   `json:"mysql_ssl_mode,omitempty" yaml:"mysql_ssl_mode,omitempty"`
	MySQLSkipSSLVerify           bool     `json:"mysql_skip_ssl_verify,omitempty" yaml:"mysql_skip_ssl_verify,omitempty"`
	
	// PostgreSQL specific fields
	PostgreSQLHost               string   `json:"postgresql_host,omitempty" yaml:"postgresql_host,omitempty"`
	PostgreSQLPort               int      `json:"postgresql_port,omitempty" yaml:"postgresql_port,omitempty"`
	PostgreSQLDatabase           string   `json:"postgresql_database,omitempty" yaml:"postgresql_database,omitempty"`
	PostgreSQLUser               string   `json:"postgresql_user,omitempty" yaml:"postgresql_user,omitempty"`
	PostgreSQLPassword           string   `json:"postgresql_password,omitempty" yaml:"postgresql_password,omitempty"`
	PostgreSQLSlotName           string   `json:"postgresql_slot_name,omitempty" yaml:"postgresql_slot_name,omitempty"`
	PostgreSQLPublicationName    string   `json:"postgresql_publication_name,omitempty" yaml:"postgresql_publication_name,omitempty"`
	PostgreSQLStartLSN           string   `json:"postgresql_start_lsn,omitempty" yaml:"postgresql_start_lsn,omitempty"`
	PostgreSQLPluginName         string   `json:"postgresql_plugin_name,omitempty" yaml:"postgresql_plugin_name,omitempty"`
	PostgreSQLStatusInterval     int      `json:"postgresql_status_interval,omitempty" yaml:"postgresql_status_interval,omitempty"`     // seconds
	PostgreSQLWalSenderTimeout   int      `json:"postgresql_wal_sender_timeout,omitempty" yaml:"postgresql_wal_sender_timeout,omitempty"` // seconds
	PostgreSQLIncludeTables      []string `json:"postgresql_include_tables,omitempty" yaml:"postgresql_include_tables,omitempty"`
	PostgreSQLExcludeTables      []string `json:"postgresql_exclude_tables,omitempty" yaml:"postgresql_exclude_tables,omitempty"`
	PostgreSQLIncludeOperations  []string `json:"postgresql_include_operations,omitempty" yaml:"postgresql_include_operations,omitempty"`
	PostgreSQLExcludeOperations  []string `json:"postgresql_exclude_operations,omitempty" yaml:"postgresql_exclude_operations,omitempty"`
	PostgreSQLIncludeSchemas     []string `json:"postgresql_include_schemas,omitempty" yaml:"postgresql_include_schemas,omitempty"`
	PostgreSQLExcludeSchemas     []string `json:"postgresql_exclude_schemas,omitempty" yaml:"postgresql_exclude_schemas,omitempty"`
	PostgreSQLMaxRetries         int      `json:"postgresql_max_retries,omitempty" yaml:"postgresql_max_retries,omitempty"`
	PostgreSQLRetryDelay         int      `json:"postgresql_retry_delay,omitempty" yaml:"postgresql_retry_delay,omitempty"`         // seconds
	PostgreSQLMaxBackoff         int      `json:"postgresql_max_backoff,omitempty" yaml:"postgresql_max_backoff,omitempty"`         // seconds
	PostgreSQLConnTimeout        int      `json:"postgresql_conn_timeout,omitempty" yaml:"postgresql_conn_timeout,omitempty"`       // seconds
	PostgreSQLMessageTimeout     int      `json:"postgresql_message_timeout,omitempty" yaml:"postgresql_message_timeout,omitempty"` // seconds
	PostgreSQLReplicationTimeout int      `json:"postgresql_replication_timeout,omitempty" yaml:"postgresql_replication_timeout,omitempty"` // seconds
	PostgreSQLSSLMode            string   `json:"postgresql_ssl_mode,omitempty" yaml:"postgresql_ssl_mode,omitempty"`
	PostgreSQLSSLCert            string   `json:"postgresql_ssl_cert,omitempty" yaml:"postgresql_ssl_cert,omitempty"`
	PostgreSQLSSLKey             string   `json:"postgresql_ssl_key,omitempty" yaml:"postgresql_ssl_key,omitempty"`
	PostgreSQLSSLRootCert        string   `json:"postgresql_ssl_root_cert,omitempty" yaml:"postgresql_ssl_root_cert,omitempty"`
	PostgreSQLSSLCrl             string   `json:"postgresql_ssl_crl,omitempty" yaml:"postgresql_ssl_crl,omitempty"`
	PostgreSQLCreateSlot         bool     `json:"postgresql_create_slot,omitempty" yaml:"postgresql_create_slot,omitempty"`
	PostgreSQLDropSlotOnExit     bool     `json:"postgresql_drop_slot_on_exit,omitempty" yaml:"postgresql_drop_slot_on_exit,omitempty"`
	PostgreSQLSlotSnapShotAction string   `json:"postgresql_slot_snapshot_action,omitempty" yaml:"postgresql_slot_snapshot_action,omitempty"`
	PostgreSQLTempSlot           bool     `json:"postgresql_temp_slot,omitempty" yaml:"postgresql_temp_slot,omitempty"`
}

type TransformOperation struct {
	Operation string                 `json:"operation"`
	Spec      map[string]interface{} `json:"spec"`
}

// Legacy Configuration type alias for backwards compatibility
type Configuration = Config

// ConfigValidator represents configuration validation interface
type ConfigValidator interface {
	// Validate validates the entire configuration
	Validate() error
	
	// ValidateStream validates a stream configuration
	ValidateStream(config StreamConfig) error
	
	// ValidateSource validates a source configuration
	ValidateSource(config SourceConfig) error
	
	// ValidateTarget validates a target configuration
	ValidateTarget(config TargetConfig) error
}

// ConfigLoader represents configuration loading interface
type ConfigLoader interface {
	// LoadFromFile loads configuration from a file
	LoadFromFile(path string) (*Config, error)
	
	// LoadFromEnv loads configuration from environment variables
	LoadFromEnv() (*Config, error)
	
	// LoadFromAzure loads configuration from Azure sources
	LoadFromAzure(ctx context.Context) (*Config, error)
	
	// Watch watches for configuration changes
	Watch(ctx context.Context, callback func(*Config)) error
}

// ConfigManager represents configuration management interface
type ConfigManager interface {
	// GetConfig returns the current configuration
	GetConfig() *Config
	
	// ReloadConfig reloads configuration from sources
	ReloadConfig(ctx context.Context) error
	
	// UpdateStream updates a stream configuration
	UpdateStream(name string, config StreamConfig) error
	
	// ValidateConfig validates configuration before applying
	ValidateConfig(config *Config) error
}

// Global is the global configuration variable
var Global *Config

// GetConfig returns the current global configuration
func GetConfig() *Config {
	return Global
}

// SetConfig sets the global configuration
func SetConfig(config *Config) {
	Global = config
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		Metrics: MetricsConfig{
			Enabled:   true,
			Port:      9090,
			Path:      "/metrics",
			Interval:  15 * time.Second,
			Namespace: "replicator",
		},
		Logging: LoggingConfig{
			Level:    "info",
			Format:   "json",
			Output:   "stdout",
			Rotation: false,
		},
		Azure: AzureConfig{
			Authentication: AuthenticationConfig{
				Method: "managed_identity",
			},
			KeyVault: KeyVaultConfig{
				Enabled: false,
			},
			CosmosDB: CosmosDBConfig{
				DefaultConsistencyLevel: "Session",
				MaxRetryAttemptsOnThrottledRequests: 3,
				MaxRetryWaitTimeInSeconds: 30,
			},
			Monitor: MonitorConfig{
				Enabled: false,
			},
		},
		OpenTelemetry: OpenTelemetryConfig{
			Enabled:     true,
			ServiceName: "replicator",
			Tracing: TracingConfig{
				Enabled:    true,
				SampleRate: 0.1,
			},
			Metrics: OTelMetricsConfig{
				Enabled:  true,
				Interval: 15 * time.Second,
			},
		},
		Telemetry: TelemetryConfig{
			Enabled:         true,
			ServiceName:     "replicator",
			ServiceVersion:  "1.0.0",
			Environment:     "development",
			MetricsEnabled:  true,
			TracingEnabled:  true,
			OTLPEndpoint:    "localhost:4317",
			MetricsInterval: 30 * time.Second,
		},
		// Legacy defaults for backwards compatibility
		Debug:              true,
		Execute:            false,
		StreamQueueLength:  10000,
		EstuaryQueueLength: 10000,
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	
	if c.Metrics.Enabled && (c.Metrics.Port <= 0 || c.Metrics.Port > 65535) {
		return fmt.Errorf("invalid metrics port: %d", c.Metrics.Port)
	}
	
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}
	
	validLogFormats := map[string]bool{
		"json": true, "text": true,
	}
	if !validLogFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}
	
	// Validate stream configurations
	streamNames := make(map[string]bool)
	for _, stream := range c.Streams {
		if stream.Name == "" {
			return fmt.Errorf("stream name cannot be empty")
		}
		if streamNames[stream.Name] {
			return fmt.Errorf("duplicate stream name: %s", stream.Name)
		}
		streamNames[stream.Name] = true
		
		if err := c.validateStreamConfig(stream); err != nil {
			return fmt.Errorf("invalid stream %s: %w", stream.Name, err)
		}
	}
	
	return nil
}

// validateStreamConfig validates a single stream configuration
func (c *Config) validateStreamConfig(stream StreamConfig) error {
	if stream.Source.Type == "" {
		return fmt.Errorf("source type cannot be empty")
	}
	
	if stream.Target.Type == "" {
		return fmt.Errorf("target type cannot be empty")
	}
	
	validSourceTypes := map[SourceType]bool{
		SourceTypeMongoDB: true, SourceTypeMySQL: true, SourceTypePostgreSQL: true,
		SourceTypeCosmosDB: true, SourceTypeKafka: true,
	}
	if !validSourceTypes[stream.Source.Type] {
		return fmt.Errorf("invalid source type: %s", stream.Source.Type)
	}
	
	validTargetTypes := map[TargetType]bool{
		TargetTypeMongoDB: true, TargetTypeMySQL: true, TargetTypePostgreSQL: true,
		TargetTypeCosmosDB: true, TargetTypeKafka: true, TargetTypeElastic: true,
	}
	if !validTargetTypes[stream.Target.Type] {
		return fmt.Errorf("invalid target type: %s", stream.Target.Type)
	}
	
	if stream.BatchSize < 0 {
		return fmt.Errorf("batch size cannot be negative")
	}
	
	if stream.BufferSize < 0 {
		return fmt.Errorf("buffer size cannot be negative")
	}
	
	return nil
}

// LoadConfiguration loads configuration using viper
func LoadConfiguration() *Config {
	viper.SetDefault("Debug", true)
	viper.SetDefault("Execute", false)
	viper.SetDefault("StreamQueueLength", 10000)
	viper.SetDefault("EstuaryQueueLength", 10000)

	// Set defaults for new configuration
	viper.SetDefault("Server.Host", "0.0.0.0")
	viper.SetDefault("Server.Port", 8080)
	viper.SetDefault("Server.ReadTimeout", "30s")
	viper.SetDefault("Server.WriteTimeout", "30s")
	viper.SetDefault("Server.ShutdownTimeout", "10s")
	
	viper.SetDefault("Metrics.Enabled", true)
	viper.SetDefault("Metrics.Port", 9090)
	viper.SetDefault("Metrics.Path", "/metrics")
	viper.SetDefault("Metrics.Interval", "15s")
	viper.SetDefault("Metrics.Namespace", "replicator")
	
	viper.SetDefault("Logging.Level", "info")
	viper.SetDefault("Logging.Format", "json")
	viper.SetDefault("Logging.Output", "stdout")
	viper.SetDefault("Logging.Rotation", false)
	
	viper.SetDefault("Azure.Authentication.Method", "managed_identity")
	viper.SetDefault("Azure.KeyVault.Enabled", false)
	viper.SetDefault("Azure.CosmosDB.DefaultConsistencyLevel", "Session")
	viper.SetDefault("Azure.CosmosDB.MaxRetryAttemptsOnThrottledRequests", 3)
	viper.SetDefault("Azure.CosmosDB.MaxRetryWaitTimeInSeconds", 30)
	viper.SetDefault("Azure.Monitor.Enabled", false)
	
	viper.SetDefault("OpenTelemetry.Enabled", true)
	viper.SetDefault("OpenTelemetry.ServiceName", "replicator")
	viper.SetDefault("OpenTelemetry.Tracing.Enabled", true)
	viper.SetDefault("OpenTelemetry.Tracing.SampleRate", 0.1)
	viper.SetDefault("OpenTelemetry.Metrics.Enabled", true)
	viper.SetDefault("OpenTelemetry.Metrics.Interval", "15s")

	viper.SetDefault("Telemetry.Enabled", true)
	viper.SetDefault("Telemetry.ServiceName", "replicator")
	viper.SetDefault("Telemetry.ServiceVersion", "1.0.0")
	viper.SetDefault("Telemetry.Environment", "development")
	viper.SetDefault("Telemetry.MetricsEnabled", true)
	viper.SetDefault("Telemetry.TracingEnabled", true)
	viper.SetDefault("Telemetry.OTLPEndpoint", "localhost:4317")
	viper.SetDefault("Telemetry.MetricsInterval", "30s")

	viper.SetConfigName("replicator.conf")   // name of config file (without extension)
	viper.AddConfigPath("/etc/replicator/")  // path to look for the config file in
	viper.AddConfigPath("$HOME/.replicator") // call multiple times to add many search paths
	viper.AddConfigPath("./conf")            // optionally look for config in the working directory
	err := viper.ReadInConfig()              // Find and read the config file
	if err != nil {                          // Handle errors reading the config file
		log.Error().Err(err).Msg("Fatal error config file")
	}

	viper.WatchConfig()
	viper.OnConfigChange(reloadConfig)
	
	cfg := DefaultConfig()
	err = viper.Unmarshal(&cfg)
	if err != nil {
		log.Error().Err(err).Msg("unable to decode into struct")
	}

	log.Debug().Msgf("configuration loaded: %+v", cfg)

	// Set log level from configuration
	level := zerolog.InfoLevel
	switch cfg.Logging.Level {
	case "debug":
		level = zerolog.DebugLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	}
	zerolog.SetGlobalLevel(level)
	
	// Legacy debug field support
	if cfg.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	
	Global = cfg
	return cfg
}

func reloadConfig(e fsnotify.Event) {
	log.Info().Msgf("Config file changed: %v", e.Name)
	cfg := DefaultConfig()
	err := viper.Unmarshal(&cfg)
	if err != nil {
		log.Error().Err(err).Msg("unable to decode into struct")
	}
	Global = cfg
}
