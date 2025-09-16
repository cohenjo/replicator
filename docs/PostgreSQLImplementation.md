# PostgreSQL Logical Replication Implementation

## Overview

This document describes the PostgreSQL logical replication stream provider implementation for the replicator project.

## Implementation Status

✅ **Completed:**
- PostgreSQL position tracking with LSN support
- Basic PostgreSQL stream provider structure
- Configuration handling for PostgreSQL connections
- Connection management for both regular and replication connections
- Replication slot creation and management
- Basic event emission structure
- Error handling and retry logic
- Position tracking integration

🚧 **Partially Implemented:**
- Message parsing (basic heartbeat implementation)
- Logical replication protocol handling (simplified version)

⏭️ **Future Enhancements:**
- Full WAL message parsing with pgoutput plugin
- Table schema detection and filtering
- Real-time change event processing
- Enhanced error handling for replication-specific errors
- Connection pooling and optimization

## Architecture

### PostgreSQL Position Type
- **File**: `pkg/position/postgres_position.go`
- **LSN Tracking**: Uses PostgreSQL's Log Sequence Number (LSN) for position tracking
- **Format**: Supports PostgreSQL's XX/XXXXXXXX LSN format
- **Features**: Timeline tracking, transaction ID support, slot name association

### Stream Provider
- **File**: `pkg/streams/postgres.go`
- **Connection Types**: 
  - Regular connection for management operations
  - Replication connection for logical replication
- **Slot Management**: Automatic replication slot creation and cleanup
- **Event Processing**: Converts PostgreSQL changes to unified RecordEvent format

## Configuration

The PostgreSQL stream provider supports extensive configuration through the global config:

```yaml
# PostgreSQL Connection Settings
postgresql_host: "localhost"
postgresql_port: 5432
postgresql_database: "mydb"
postgresql_user: "replicator"
postgresql_password: "password"

# Replication Settings
postgresql_slot_name: "replicator_slot"
postgresql_publication_name: "replicator_publication"
postgresql_plugin_name: "pgoutput"
postgresql_start_lsn: "0/0"

# SSL Settings
postgresql_ssl_mode: "prefer"
postgresql_ssl_cert: "/path/to/cert.pem"
postgresql_ssl_key: "/path/to/key.pem"

# Advanced Settings
postgresql_create_slot: true
postgresql_drop_slot_on_exit: false
postgresql_temp_slot: false
```

## Dependencies

- **github.com/jackc/pgx/v5**: PostgreSQL driver and connection management
- **github.com/jackc/pglogrepl**: PostgreSQL logical replication protocol support
- **github.com/jackc/pgconn**: Low-level PostgreSQL connection handling

## Usage Example

```go
eventSender := make(chan events.RecordEvent, 100)
logger := logrus.New()

// Create PostgreSQL stream provider
provider := NewPostgreSQLStreamProvider(eventSender, logger)

// Start listening in a goroutine
go func() {
    if err := provider.Listen(context.Background()); err != nil {
        logger.WithError(err).Error("PostgreSQL stream failed")
    }
}()

// Process events
for event := range eventSender {
    fmt.Printf("Received PostgreSQL event: %+v\n", event)
}
```

## PostgreSQL Prerequisites

1. **Enable Logical Replication:**
   ```sql
   -- Set wal_level to logical
   ALTER SYSTEM SET wal_level = logical;
   
   -- Set max_replication_slots (restart required)
   ALTER SYSTEM SET max_replication_slots = 4;
   
   -- Restart PostgreSQL server
   ```

2. **Create Publication:**
   ```sql
   -- Create publication for all tables
   CREATE PUBLICATION replicator_publication FOR ALL TABLES;
   
   -- Or for specific tables
   CREATE PUBLICATION replicator_publication FOR TABLE users, orders;
   ```

3. **Grant Permissions:**
   ```sql
   -- Grant replication permissions
   ALTER USER replicator REPLICATION;
   
   -- Grant table access
   GRANT SELECT ON ALL TABLES IN SCHEMA public TO replicator;
   ```

## Position Tracking Integration

The PostgreSQL implementation integrates with the replicator's position tracking system:

- **Automatic Position Saving**: Saves LSN positions periodically
- **Resume Support**: Can resume from last saved position
- **Multiple Backends**: Supports file, database, MongoDB, and Azure Storage backends
- **Metadata**: Stores connection and slot information with positions

## Testing

Basic unit tests are provided in `pkg/streams/postgres_test.go`:

```bash
# Run PostgreSQL stream tests
go test ./pkg/streams -v -run TestPostgreSQL

# Run position tests  
go test ./pkg/position -v -run TestPostgreSQL
```

## Error Handling

The implementation includes comprehensive error handling:

- **Fatal Errors**: Authentication, permission, and configuration errors
- **Retryable Errors**: Network timeouts and temporary connection issues
- **Exponential Backoff**: Progressive retry delays for resilience
- **Connection Recovery**: Automatic reconnection on connection loss

## Monitoring

PostgreSQL streams emit metrics and logs for monitoring:

- **Events Processed**: Count of replication events processed
- **LSN Progress**: Current LSN position tracking
- **Connection Status**: Replication connection health
- **Error Rates**: Failed operation tracking

## Performance Considerations

- **Message Batching**: Process multiple changes in batches
- **Position Caching**: Cache positions to reduce save frequency
- **Connection Pooling**: Reuse connections for efficiency
- **Memory Management**: Stream large datasets without memory issues

## Integration with Replicator Core

The PostgreSQL stream provider integrates with:

1. **Event System**: Emits `events.RecordEvent` for downstream processing
2. **Position Tracking**: Uses `position.Tracker` for resume capability  
3. **Configuration**: Reads from global `config.Config`
4. **Metrics**: Reports to Prometheus metrics system
5. **Logging**: Uses structured logging with logrus

## Future Development

Areas for enhancement in future iterations:

1. **Message Parsing**: Full implementation of pgoutput message parsing
2. **Schema Evolution**: Handle DDL changes and schema updates
3. **Conflict Resolution**: Handle replication conflicts and resolution
4. **Performance Optimization**: Parallel processing and optimizations
5. **Advanced Filtering**: More sophisticated table and operation filtering