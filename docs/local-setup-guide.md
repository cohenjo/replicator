# Local Setup Guide

This guide provides step-by-step instructions for running Replicator scenarios locally, including MySQL-to-Elasticsearch and MongoDB-to-MongoDB replication.

## Prerequisites

- Docker and Docker Compose installed
- Go 1.21+ installed
- Git installed

## Initial Setup

### 1. Clone and Build

```bash
git clone https://github.com/cohenjo/replicator.git
cd replicator
go build -o replicator ./cmd/replicator
```

### 2. Start Infrastructure

Start all required services (databases, Elasticsearch, etc.):

```bash
docker-compose up -d
```

This will start:
- MySQL Source (port 3306) and Target (port 3307)
- MongoDB Source (port 27017) and Target (port 27018) 
- Elasticsearch (port 9200)
- Supporting services (Kafka, Redis, Prometheus, Grafana)

### 3. Wait for Services

Wait for all services to be ready:

```bash
sleep 30
```

## Scenario 1: MySQL to Elasticsearch Replication

### Overview
This scenario demonstrates streaming MySQL binary log changes to Elasticsearch for real-time search indexing.

### Setup Steps

#### 1. Initialize MongoDB Replica Sets (Required for Change Streams)

MongoDB requires replica sets for change streams. Configure them:

```bash
# Initialize source MongoDB replica set
docker exec replicator-mongodb-source mongosh --username admin --password password123 --authenticationDatabase admin --eval "rs.initiate({_id: 'src-rs', members: [{_id: 0, host: 'localhost:27017'}]})"

# Initialize target MongoDB replica set  
docker exec replicator-mongodb-target mongosh --username admin --password password123 --authenticationDatabase admin --eval "rs.initiate({_id: 'tgt-rs', members: [{_id: 0, host: 'localhost:27017'}]})"

# Reconfigure for external access
docker exec replicator-mongodb-source mongosh --username admin --password password123 --authenticationDatabase admin --eval "cfg = rs.config(); cfg.members[0].host = 'localhost:27017'; rs.reconfig(cfg, {force: true})"
docker exec replicator-mongodb-target mongosh --username admin --password password123 --authenticationDatabase admin --eval "cfg = rs.config(); cfg.members[0].host = 'localhost:27017'; rs.reconfig(cfg, {force: true})"
```

#### 2. Setup Elasticsearch Index

Create the products index with proper mapping:

```bash
./examples/elasticsearch/setup-products-index.sh
```

#### 3. Verify Source Data

Check that MySQL has sample data:

```bash
docker exec replicator-mysql-source mysql -u replicator -ppassword123 source_db -e "SELECT COUNT(*) FROM products;"
```

#### 4. Start Replication

```bash
./replicator --config ./examples/configs/mysql-to-elasticsearch-new.yaml
```

#### 5. Test Real-time Replication

In another terminal, add new data to MySQL:

```bash
docker exec replicator-mysql-source mysql -u replicator -ppassword123 source_db -e "INSERT INTO products (name, price, category_id) VALUES ('Test Product', 99.99, 1);"
```

#### 6. Verify Data in Elasticsearch

Check that data appears in Elasticsearch:

```bash
curl -X GET "localhost:9200/products/_search?pretty"
```

### Expected Output

You should see:
- Replicator logs showing binlog events being processed
- New product appearing in Elasticsearch with generic field mapping (field_0, field_1, etc.)
- Real-time replication with sub-second latency

## Scenario 2: MongoDB to MongoDB Replication

### Overview
This scenario demonstrates MongoDB change stream replication between two MongoDB instances.

### Setup Steps

#### 1. Verify Replica Sets (if not done in Scenario 1)

```bash
# Check source replica set status
docker exec replicator-mongodb-source mongosh --username admin --password password123 --authenticationDatabase admin --eval "rs.status()"

# Check target replica set status
docker exec replicator-mongodb-target mongosh --username admin --password password123 --authenticationDatabase admin --eval "rs.status()"
```

#### 2. Verify Source Data

Check that MongoDB source has sample user data:

```bash
docker exec replicator-mongodb-source mongosh --username admin --password password123 --authenticationDatabase admin source_db --eval "db.users.countDocuments()"
```

#### 3. Start Replication

```bash
./replicator --config ./examples/configs/mongodb-to-mongodb-bridge.yaml
```

#### 4. Test Real-time Replication

In another terminal, add new data to source MongoDB:

```bash
docker exec replicator-mongodb-source mongosh --username admin --password password123 --authenticationDatabase admin source_db --eval "db.users.insertOne({name: 'Test User', email: 'test@example.com', created_at: new Date()})"
```

#### 5. Verify Data in Target

Check that data appears in target MongoDB:

```bash
docker exec replicator-mongodb-target mongosh --username admin --password password123 --authenticationDatabase admin target_db --eval "db.users.find({name: 'Test User'}).pretty()"
```

### Expected Output

You should see:
- Replicator logs showing change stream events being processed
- New user document appearing in target MongoDB
- Real-time replication using change streams

## Architecture Validation

Both scenarios demonstrate:

### ✅ Generic Field Mapping
- No hardcoded schemas required
- Dynamic field mapping (field_0, field_1, etc.)
- Works across different database types

### ✅ Stream Interface
- MySQL BinlogStream implements models.Stream
- MongoDB MongoDBStream implements models.Stream  
- Consistent interface across different sources

### ✅ EstuaryBridge Pattern
- Bridges new stream system with legacy endpoints
- Supports multiple target types (Elasticsearch, MongoDB)
- Configuration format conversion

### ✅ Real-time Processing
- Sub-second replication latency
- Change stream and binlog processing
- Event transformation pipeline

## Configuration Files

### MySQL to Elasticsearch
- **File**: `examples/configs/mysql-to-elasticsearch-new.yaml`
- **Source**: MySQL with binlog enabled
- **Target**: Elasticsearch with products index
- **Features**: Binary log streaming, field mapping

### MongoDB to MongoDB
- **File**: `examples/configs/mongodb-to-mongodb-bridge.yaml`
- **Source**: MongoDB with change streams
- **Target**: MongoDB with direct connection
- **Features**: Change stream processing, replica set support

## Troubleshooting

### Common Issues

#### MySQL Replication
- **Binlog not enabled**: Ensure MySQL is configured with `--binlog-format=ROW`
- **Permission errors**: Verify MySQL user has REPLICATION privileges
- **Connection issues**: Check MySQL is accessible on port 3306

#### MongoDB Replication
- **Replica set required**: Change streams only work with replica sets
- **Authentication errors**: Ensure credentials and authSource=admin
- **Connection timeouts**: Use directConnection=true for target

#### Elasticsearch
- **Index mapping**: Run setup script to create proper index
- **Connection refused**: Ensure Elasticsearch is running on port 9200
- **Security disabled**: Configuration disables X-Pack security

### Debug Mode

Enable debug logging for detailed output:

```yaml
logging:
  level: "debug"
  format: "json"
```

### Monitoring

Access monitoring interfaces:
- **Prometheus**: http://localhost:9091
- **Grafana**: http://localhost:3000 (admin/admin123)
- **Replicator API**: http://localhost:8080/health

## Cleanup

Stop all services:

```bash
docker-compose down -v
```

This removes all containers and volumes, completely cleaning the environment.

## Next Steps

- Try different source/target combinations
- Explore transformation rules
- Set up monitoring and alerting
- Deploy to production environments

For more advanced configurations, see the [Configuration Reference](configuration.md).