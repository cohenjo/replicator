# MongoDB Position Storage Configuration Examples

## Example 1: Simple MongoDB Configuration
```yaml
position_tracking:
  type: "mongodb"
  stream_id: "mysql-production-db"
  mongo:
    connection_uri: "mongodb://localhost:27017"
    database: "replicator_positions"
    collection: "stream_positions"
```

## Example 2: Production MongoDB Configuration with Replica Set
```yaml
position_tracking:
  type: "mongodb"
  stream_id: "mysql-production-db"
  mongo:
    connection_uri: "mongodb://user:pass@mongo1:27017,mongo2:27017,mongo3:27017/replicator?replicaSet=rs0&authSource=admin"
    database: "replicator_positions"
    collection: "stream_positions"
    connect_timeout: 10s
    server_selection_timeout: 30s
    socket_timeout: 10s
    max_pool_size: 100
    min_pool_size: 5
    read_concern: "majority"
    enable_transactions: true
    enable_auto_index_creation: true
    retry_writes: true
    retry_reads: true
    compressors: ["zlib", "snappy"]
    write_concern:
      w: "majority"
      j: true
      wtimeout: 5s
```

## Example 3: Database Tracker Configuration (Auto-Maps to MongoDB)
```yaml
position_tracking:
  type: "database"
  stream_id: "mysql-production-db"
  database:
    type: "mongodb"
    connection_string: "mongodb://localhost:27017"
    schema: "replicator_positions"
    collection_name: "stream_positions"
    use_transactions: true
    enable_auto_migration: true
    connection_pool_size: 50
    connection_timeout: 10s
```

## Example 4: Multiple Position Storage Options
```yaml
# File-based position tracking
position_tracking_file:
  type: "file"
  stream_id: "mysql-dev-db"
  file:
    directory: "/var/lib/replicator/positions"
    enable_backup: true
    backup_count: 5

# MongoDB position tracking
position_tracking_mongo:
  type: "mongodb"
  stream_id: "mysql-prod-db"
  mongo:
    connection_uri: "mongodb://mongo-cluster:27017"
    database: "replicator_positions"
    enable_transactions: true

# Azure Storage position tracking (when available)
position_tracking_azure:
  type: "azure"
  stream_id: "mysql-cloud-db"
  azure:
    account_name: "replicatorstore"
    container_name: "positions"
    blob_prefix: "prod/"
```

## Example 5: Stream Provider Integration
```yaml
streams:
  mysql_production:
    type: "mysql"
    host: "mysql-prod.example.com"
    port: 3306
    username: "replicator"
    password: "${MYSQL_PASSWORD}"
    
    # MongoDB position tracking for this stream
    position_tracking:
      type: "mongodb"
      mongo:
        connection_uri: "mongodb://mongo-cluster:27017/replicator?replicaSet=rs0"
        database: "stream_positions"
        collection: "mysql_positions"
        enable_transactions: true
        write_concern:
          w: "majority"
          j: true
          wtimeout: 3s

  mysql_staging:
    type: "mysql"
    host: "mysql-staging.example.com"
    port: 3306
    username: "replicator"
    password: "${MYSQL_PASSWORD}"
    
    # File-based position tracking for staging
    position_tracking:
      type: "file"
      file:
        directory: "/tmp/positions"
```

## Example 6: High Availability Configuration
```yaml
position_tracking:
  type: "mongodb"
  stream_id: "critical-production-stream"
  
  # Retry configuration
  retry_attempts: 3
  retry_delay: 1s
  update_interval: 5s
  
  mongo:
    # MongoDB cluster with authentication
    connection_uri: "mongodb://replicator:${MONGO_PASSWORD}@mongo1:27017,mongo2:27017,mongo3:27017/replicator?replicaSet=rs0&authSource=admin&ssl=true"
    database: "replicator_positions"
    collection: "critical_positions"
    
    # Connection settings for HA
    connect_timeout: 5s
    server_selection_timeout: 10s
    socket_timeout: 30s
    max_pool_size: 200
    min_pool_size: 10
    
    # Consistency settings
    read_concern: "majority"
    enable_transactions: true
    retry_writes: true
    retry_reads: true
    
    # Performance settings
    compressors: ["zstd", "zlib"]
    
    # Durability settings
    write_concern:
      w: "majority"
      j: true
      wtimeout: 10s
    
    # Operations settings
    enable_auto_index_creation: true
```

## Environment Variables

For security, use environment variables for sensitive information:

```bash
# MongoDB credentials
export MONGO_PASSWORD="secure_password"
export MONGO_URI="mongodb://replicator:${MONGO_PASSWORD}@mongo-cluster:27017/replicator?replicaSet=rs0"

# Application configuration
export REPLICATOR_POSITION_TYPE="mongodb"
export REPLICATOR_POSITION_DATABASE="replicator_positions"
```

## Connection String Examples

### Local Development
```
mongodb://localhost:27017
```

### Authenticated Single Server
```
mongodb://username:password@localhost:27017/database
```

### Replica Set with Authentication
```
mongodb://username:password@host1:27017,host2:27017,host3:27017/database?replicaSet=myReplSet&authSource=admin
```

### SSL/TLS Enabled
```
mongodb://username:password@host1:27017,host2:27017,host3:27017/database?replicaSet=myReplSet&ssl=true&authSource=admin
```

### MongoDB Atlas (Cloud)
```
mongodb+srv://username:password@cluster.mongodb.net/database?retryWrites=true&w=majority
```

## Performance Tuning

### High Throughput Configuration
```yaml
mongo:
  max_pool_size: 500
  min_pool_size: 50
  socket_timeout: 60s
  compressors: ["zstd"]
  write_concern:
    w: 1  # Faster writes, less durability
    j: false
    wtimeout: 1s
```

### High Durability Configuration
```yaml
mongo:
  enable_transactions: true
  read_concern: "majority"
  write_concern:
    w: "majority"
    j: true
    wtimeout: 30s
```

### Low Latency Configuration
```yaml
mongo:
  connect_timeout: 1s
  server_selection_timeout: 2s
  socket_timeout: 5s
  read_concern: "local"
  write_concern:
    w: 1
    j: false
    wtimeout: 100ms
```