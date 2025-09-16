# Data Model: Database Replication System

## Core Entities

### Configuration File
**Purpose**: Defines replication streams and operational parameters
**Fields**:
- `version`: string - Configuration schema version
- `service`: ServiceConfiguration - Service-level settings
- `streams`: []StreamConfiguration - Array of replication stream definitions
- `authentication`: []AuthConfiguration - Authentication provider configurations
- `transformations`: []TransformationRule - Reusable transformation definitions

**Validation Rules**:
- Version must match supported schema versions
- At least one stream must be defined
- All referenced authentication and transformation IDs must exist
- No circular dependencies in stream configurations

### StreamConfiguration
**Purpose**: Defines a single replication stream from source to destination
**Fields**:
- `id`: string - Unique stream identifier
- `name`: string - Human-readable stream name
- `source`: SourceConfiguration - Source database configuration
- `destination`: DestinationConfiguration - Destination database configuration
- `transformation_id`: string (optional) - Reference to transformation rule
- `enabled`: bool - Whether stream is active
- `batch_size`: int - Number of events to process in batch
- `retry_policy`: RetryPolicy - Error handling configuration

**Validation Rules**:
- ID must be unique across all streams
- Source and destination must not be identical
- Batch size must be positive integer
- Transformation ID must reference valid transformation if specified

### SourceConfiguration
**Purpose**: Defines source database connection and change stream settings
**Fields**:
- `type`: string - Database type (mongodb, mysql, postgres, cosmosdb)
- `connection_string`: string - Database connection details
- `auth_id`: string - Reference to authentication configuration
- `database`: string - Database name to monitor
- `collection_table`: string (optional) - Specific collection/table filter
- `change_stream_options`: map[string]interface{} - Database-specific options

**Validation Rules**:
- Type must be supported database type
- Connection string format must match database type requirements
- Auth ID must reference valid authentication configuration
- Database name is required for all types

### DestinationConfiguration
**Purpose**: Defines destination database connection and write settings
**Fields**:
- `type`: string - Database type (mongodb, mysql, postgres, cosmosdb)
- `connection_string`: string - Database connection details
- `auth_id`: string - Reference to authentication configuration
- `database`: string - Target database name
- `collection_table`: string (optional) - Target collection/table name
- `write_options`: map[string]interface{} - Database-specific write options

**Validation Rules**:
- Type must be supported database type
- Connection string format must match database type requirements
- Auth ID must reference valid authentication configuration
- Database name is required for all types

### AuthConfiguration
**Purpose**: Defines authentication methods for database connections
**Fields**:
- `id`: string - Unique authentication identifier
- `type`: string - Authentication type (azure_entra, username_password, certificate)
- `config`: map[string]interface{} - Type-specific authentication parameters

**Validation Rules**:
- ID must be unique across all authentication configurations
- Type must be supported authentication method
- Config must contain required fields for authentication type

### TransformationRule
**Purpose**: Defines data transformation logic for events
**Fields**:
- `id`: string - Unique transformation identifier
- `name`: string - Human-readable transformation name
- `type`: string - Transformation engine type (kazaam, custom)
- `specification`: map[string]interface{} - Transformation logic definition
- `error_handling`: ErrorHandling - How to handle transformation errors

**Validation Rules**:
- ID must be unique across all transformation rules
- Type must be supported transformation engine
- Specification must be valid for transformation type

### ReplicationEvent
**Purpose**: Represents a single database change event during processing
**Fields**:
- `stream_id`: string - Source stream identifier
- `event_id`: string - Unique event identifier
- `operation_type`: string - Type of operation (insert, update, delete)
- `timestamp`: time.Time - When the change occurred
- `source_position`: string - Position in source change stream
- `document_id`: string - Identifier of changed document/record
- `before_data`: map[string]interface{} (optional) - Document state before change
- `after_data`: map[string]interface{} (optional) - Document state after change
- `metadata`: map[string]interface{} - Additional event metadata

**State Transitions**:
- Received → Transforming (if transformation required)
- Received → Writing (if no transformation)
- Transforming → Writing (transformation completed)
- Transforming → Failed (transformation error)
- Writing → Completed (successfully written)
- Writing → Failed (write error)
- Failed → Retrying (retry attempt)

### ReplicationPosition
**Purpose**: Tracks processing position for stream recovery
**Fields**:
- `stream_id`: string - Associated stream identifier
- `position`: string - Database-specific position token
- `timestamp`: time.Time - When position was last updated
- `event_count`: int64 - Number of events processed at this position
- `checksum`: string - Validation checksum for position integrity

**Validation Rules**:
- Stream ID must reference valid stream configuration
- Position format must be valid for source database type
- Timestamp must not be in the future
- Event count must be non-negative

### ServiceConfiguration
**Purpose**: Service-level operational parameters
**Fields**:
- `log_level`: string - Logging verbosity level
- `log_format`: string - Log output format (json, text)
- `metrics_port`: int - Port for metrics endpoint
- `health_port`: int - Port for health check endpoint
- `graceful_shutdown_timeout`: duration - Timeout for graceful shutdown
- `max_concurrent_streams`: int - Maximum concurrent stream processing

**Validation Rules**:
- Log level must be valid logging level
- Port numbers must be in valid range (1024-65535)
- Timeout values must be positive
- Max concurrent streams must be positive integer

### RetryPolicy
**Purpose**: Defines error handling and retry behavior
**Fields**:
- `max_attempts`: int - Maximum retry attempts
- `initial_delay`: duration - Initial retry delay
- `max_delay`: duration - Maximum retry delay
- `backoff_multiplier`: float64 - Exponential backoff multiplier
- `permanent_error_codes`: []string - Error codes that skip retry

**Validation Rules**:
- Max attempts must be positive integer
- Delays must be positive durations
- Backoff multiplier must be >= 1.0
- Error codes must be database-specific valid codes

### ErrorHandling
**Purpose**: Transformation error handling configuration
**Fields**:
- `on_error`: string - Action on error (skip, retry, fail_stream, dead_letter)
- `dead_letter_destination`: string (optional) - Where to send failed events
- `max_retries`: int - Maximum transformation retry attempts
- `retry_delay`: duration - Delay between transformation retries

**Validation Rules**:
- On error action must be supported value
- Dead letter destination required if action is dead_letter
- Max retries must be non-negative
- Retry delay must be positive duration

## Relationships

### Configuration Hierarchy
```
Configuration File
├── Service Configuration (1:1)
├── Authentication Configurations (1:N)
├── Transformation Rules (1:N)
└── Stream Configurations (1:N)
    ├── Source Configuration (1:1)
    ├── Destination Configuration (1:1)
    ├── Authentication Reference (N:1)
    └── Transformation Reference (N:1, optional)
```

### Runtime Data Flow
```
Source Database Change
└── Replication Event
    ├── Associated Stream Configuration (N:1)
    ├── Transformation Rule (N:1, optional)
    └── Replication Position (1:1)
```

### Position Tracking
```
Stream Configuration (1:1) ←→ Replication Position
Replication Position (1:N) ←→ Checkpoint History
```

## Data Persistence Requirements

### Configuration Data
- **Storage**: File system (YAML/JSON files)
- **Backup**: Version control system
- **Validation**: Schema validation on load
- **Hot Reload**: File system watcher for updates

### Position Data
- **Storage**: Persistent key-value store or database
- **Durability**: ACID properties required
- **Performance**: Sub-millisecond read/write latency
- **Backup**: Regular snapshots with point-in-time recovery

### Event Data (In-Transit)
- **Storage**: Memory queues with overflow to disk
- **Retention**: Until successfully processed or failed permanently
- **Ordering**: Maintain source order within streams
- **Batching**: Configurable batch sizes for efficiency

### Metrics Data
- **Storage**: Time-series metrics system (Prometheus/OpenTelemetry)
- **Retention**: Configurable based on operational needs
- **Aggregation**: Real-time aggregation for dashboards
- **Export**: Multiple export formats supported