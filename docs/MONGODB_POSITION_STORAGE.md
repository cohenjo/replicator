# MongoDB Position Storage Implementation

## Overview

Added comprehensive MongoDB position storage support to the replicator's position tracking system. This provides a robust, scalable, and feature-rich option for persisting stream positions in MongoDB.

## Implementation Details

### 1. MongoDB Position Tracker (`mongo_tracker.go`)

A full-featured MongoDB-based position tracker that implements the `Tracker` interface with:

#### Key Features:
- **Connection Management**: Robust MongoDB connection with configurable timeouts, pooling, and authentication
- **Transaction Support**: Optional atomic operations using MongoDB transactions
- **Write Concerns**: Configurable write concerns (majority, numeric, custom tags) with journal acknowledgment
- **Read Concerns**: Support for different consistency levels (local, available, majority, linearizable, snapshot)
- **Automatic Indexing**: Optional creation of optimized indexes for performance
- **Health Monitoring**: Built-in health checks and collection statistics
- **Compression**: Network compression support (zlib, zstd, snappy)
- **Error Handling**: Comprehensive error handling with proper MongoDB error classification

#### Configuration Structure:
```go
type MongoConfig struct {
    ConnectionURI              string
    Database                   string
    Collection                 string
    ConnectTimeout            time.Duration
    ServerSelectionTimeout    time.Duration
    SocketTimeout             time.Duration
    MaxPoolSize               uint64
    MinPoolSize               uint64
    ReadConcern               string
    WriteConcern              *MongoWriteConcern
    EnableTransactions        bool
    EnableAutoIndexCreation   bool
    RetryWrites              bool
    RetryReads               bool
    Compressors              []string
}
```

#### Document Schema:
```go
type MongoPositionDocument struct {
    ID           string                 `bson:"_id"`           // streamID as document ID
    StreamID     string                 `bson:"stream_id"`
    PositionData []byte                 `bson:"position_data"`
    Metadata     map[string]interface{} `bson:"metadata"`
    CreatedAt    time.Time              `bson:"created_at"`
    UpdatedAt    time.Time              `bson:"updated_at"`
    Version      int64                  `bson:"version"`       // For optimistic locking
}
```

### 2. Enhanced Database Tracker (`database_tracker.go`)

Updated the database tracker to serve as a factory for different database types:

#### Supported Database Types:
- **MongoDB**: Full implementation with comprehensive configuration options
- **MySQL**: Placeholder for future implementation
- **PostgreSQL**: Placeholder for future implementation

#### Factory Pattern:
- Automatic creation of appropriate tracker based on database type
- Configuration mapping from generic `DatabaseConfig` to specific tracker configs
- Delegation pattern for clean separation of concerns

### 3. Updated Configuration System (`position.go`)

Enhanced the main configuration structures to support MongoDB:

#### New Configuration Fields:
- Added `MongoConfig` to main `Config` struct
- Added `MongoConfig` to `DatabaseConfig` for nested configuration
- Updated factory function `NewTracker()` to support "mongodb" and "mongo" types

### 4. Comprehensive Test Suite (`mongo_tracker_test.go`)

Extensive test coverage including:

#### Configuration Tests:
- Validation of required fields
- Default value application
- Error handling for invalid configurations

#### Integration Tests (MongoDB-dependent):
- Connection and health checks
- Save/Load operations with metadata
- Delete operations
- List all positions
- Transaction support
- Concurrent access patterns
- Write concern variations
- Collection statistics

#### Graceful Degradation:
- Tests skip automatically when MongoDB is not available
- No false failures in CI/CD environments

## Usage Examples

### 1. Direct MongoDB Tracker
```go
config := &MongoConfig{
    ConnectionURI: "mongodb://localhost:27017",
    Database:      "replicator_positions",
    Collection:    "stream_positions",
    EnableTransactions: true,
    EnableAutoIndexCreation: true,
    WriteConcern: &MongoWriteConcern{
        W:        "majority",
        J:        true,
        WTimeout: 5 * time.Second,
    },
}

tracker, err := NewMongoTracker(config)
if err != nil {
    log.Fatal(err)
}
defer tracker.Close()
```

### 2. Via Database Tracker Factory
```go
config := &DatabaseConfig{
    Type:              "mongodb",
    ConnectionString:  "mongodb://localhost:27017",
    Schema:           "replicator_positions",
    CollectionName:   "stream_positions",
    UseTransactions:  true,
    EnableAutoMigration: true,
}

tracker, err := NewDatabaseTracker(config)
if err != nil {
    log.Fatal(err)
}
defer tracker.Close()
```

### 3. Via Main Position Factory
```go
config := &Config{
    Type: "mongodb",
    MongoConfig: &MongoConfig{
        ConnectionURI: "mongodb://localhost:27017",
        Database:      "replicator_positions",
    },
}

tracker, err := NewTracker(config)
if err != nil {
    log.Fatal(err)
}
defer tracker.Close()
```

### 4. Position Operations
```go
// Save position
position := &MySQLPosition{
    File:     "mysql-bin.000001",
    Position: 1234,
    GTID:     "uuid:1-10",
}

metadata := map[string]interface{}{
    "stream_type": "mysql",
    "host":        "mysql-server",
    "port":        3306,
}

err = tracker.Save(ctx, "mysql-stream-1", position, metadata)

// Load position
position, metadata, err := tracker.Load(ctx, "mysql-stream-1")

// Delete position
err = tracker.Delete(ctx, "mysql-stream-1")

// List all positions
positions, err := tracker.List(ctx)
```

## Performance Optimizations

### 1. Automatic Indexes
When `EnableAutoIndexCreation` is true, the following indexes are created:
- `stream_id_unique`: Unique index on stream_id field
- `updated_at_desc`: Descending index for timestamp queries
- `stream_type_idx`: Index on metadata.stream_type for filtering
- `created_at_asc`: Ascending index for creation time queries

### 2. Connection Pooling
- Configurable min/max pool sizes
- Connection timeout management
- Automatic connection recycling

### 3. Write Optimization
- Atomic upsert operations
- Optional transaction support for consistency
- Configurable write concerns for performance vs durability trade-offs

### 4. Read Optimization
- Configurable read concerns for consistency requirements
- Efficient document retrieval with minimal data transfer
- Bulk operations for listing multiple positions

## Production Considerations

### 1. High Availability
- Connection string supports replica sets
- Automatic failover with proper read/write concern configuration
- Network compression for reduced bandwidth usage

### 2. Security
- Connection URI supports authentication mechanisms
- TLS/SSL support through connection string
- No sensitive data in logs

### 3. Monitoring
- Built-in health checks
- Collection statistics via `GetStats()`
- Comprehensive error logging with structured fields

### 4. Scalability
- Horizontal scaling through MongoDB sharding
- Efficient indexing for large position collections
- Connection pooling for high-concurrency scenarios

## Integration Points

### 1. Stream Providers
Any stream provider can use MongoDB position tracking:
```go
// In MySQL stream provider
tracker, err := position.NewTracker(&position.Config{
    Type: "mongodb",
    MongoConfig: &position.MongoConfig{
        ConnectionURI: "mongodb://localhost:27017",
        Database:      "mysql_positions",
    },
})
```

### 2. Configuration System
MongoDB configuration integrates with existing config patterns:
```yaml
position_tracking:
  type: mongodb
  mongo:
    connection_uri: "mongodb://localhost:27017"
    database: "replicator_positions"
    collection: "stream_positions"
    enable_transactions: true
    enable_auto_index_creation: true
    write_concern:
      w: "majority"
      j: true
      wtimeout: 5s
```

### 3. Multiple Storage Options
Users can now choose from:
- **File Storage**: Simple, local file-based tracking
- **Azure Storage**: Cloud blob storage for distributed deployments
- **MongoDB Storage**: Database storage for enterprise scenarios

## Testing Status

✅ **Configuration Validation**: All edge cases covered
✅ **Unit Tests**: Core functionality thoroughly tested
✅ **Integration Tests**: Real MongoDB operations (when available)
✅ **Error Handling**: Comprehensive error scenarios
✅ **Concurrency**: Multi-threaded access patterns
✅ **Performance**: Write concern and transaction variations

## Future Enhancements

1. **MySQL/PostgreSQL Support**: Complete the database tracker with SQL database implementations
2. **Position Type Registry**: Automatic position deserialization based on stream type
3. **Metrics Integration**: OpenTelemetry metrics for position tracking operations
4. **Backup/Restore**: Built-in position backup and restoration capabilities
5. **Migration Tools**: Tools for migrating positions between storage types

## Summary

The MongoDB position storage implementation provides a production-ready, enterprise-grade solution for position tracking with:

- **Comprehensive Configuration**: All MongoDB features accessible
- **Robust Error Handling**: Graceful failure handling and recovery
- **High Performance**: Optimized for both throughput and consistency
- **Easy Integration**: Seamless integration with existing position tracking system
- **Future-Proof**: Extensible design for additional database types

This implementation completes the requested generic position tracking system with "entry points to support multiple options of tracking" including file storage, Azure Storage, and now MongoDB storage.