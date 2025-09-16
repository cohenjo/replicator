# Quickstart Guide: Database Replication System

This guide walks you through setting up and running the database replication system locally using Docker Compose, demonstrating various replication scenarios.

## Prerequisites

- Docker and Docker Compose
- Git
- 8GB+ RAM available for containers
- Ports 3000, 5432-5433, 6379, 8080, 9090-9092, 9200, 27017-27018 available

## 🚀 Quick Start (5 minutes)

### 1. Clone and Setup

```bash
git clone <repository-url>
cd replicator
```

### 2. Run Complete Setup

```bash
# Check prerequisites and start all services
./quickstart.sh start

# This will:
# - Build the replicator application
# - Start all databases (MongoDB, MySQL, PostgreSQL)
# - Start Elasticsearch, Kafka, Redis
# - Start monitoring (Prometheus, Grafana)
# - Initialize sample data
```

### 3. Run Your First Replication

```bash
# Start MongoDB to MongoDB replication
./quickstart.sh run mongodb-to-mongodb

# Test the replication with sample data
./quickstart.sh test mongodb-to-mongodb

# Monitor the logs
./quickstart.sh logs
```

### 4. Explore the Environment

Access the web interfaces:
- **Replicator API**: http://localhost:8080/health
- **Replicator Metrics**: http://localhost:9090/metrics
- **Grafana Dashboards**: http://localhost:3000 (admin/admin123)
- **Prometheus**: http://localhost:9091

## 📊 Available Replication Scenarios

### 1. MongoDB to MongoDB
**Use case**: Basic document replication between MongoDB instances
```bash
./quickstart.sh run mongodb-to-mongodb
./quickstart.sh test mongodb-to-mongodb
```

**What it demonstrates**:
- Change stream processing
- Document transformation (field mapping)
- Position tracking
- Error handling with retries

### 2. MySQL to Elasticsearch
**Use case**: Real-time search indexing from transactional data
```bash
./quickstart.sh run mysql-to-elasticsearch
./quickstart.sh test mysql-to-elasticsearch
```

**What it demonstrates**:
- Binary log (binlog) processing
- Data enrichment and transformation
- Full-text search preparation
- Bulk indexing optimization

### 3. PostgreSQL to Kafka
**Use case**: Change data capture streaming for event-driven architecture
```bash
./quickstart.sh run postgresql-to-kafka
./quickstart.sh test postgresql-to-kafka
```

**What it demonstrates**:
- Logical replication slots
- Change event enveloping (Debezium format)
- Kafka topic partitioning
- Dead letter queue handling

### 4. Multi-Source Aggregation
**Use case**: Combining data from multiple sources into analytics platform
```bash
./quickstart.sh run multi-source-aggregation
./quickstart.sh test multi-source-aggregation
```

**What it demonstrates**:
- Multiple concurrent streams
- Data aggregation patterns
- Cross-system analytics
- Unified monitoring

## 🔧 Service Endpoints

| Service | URL | Credentials |
|---------|-----|-------------|
| **Application** |
| Replicator API | http://localhost:8080 | - |
| Health Check | http://localhost:8080/health | - |
| Metrics | http://localhost:9090/metrics | - |
| **Monitoring** |
| Grafana | http://localhost:3000 | admin/admin123 |
| Prometheus | http://localhost:9091 | - |
| **Source Databases** |
| MongoDB | localhost:27017 | admin/password123 |
| MySQL | localhost:3306 | replicator/password123 |
| PostgreSQL | localhost:5432 | replicator/password123 |
| **Target Systems** |
| MongoDB Target | localhost:27018 | admin/password123 |
| MySQL Target | localhost:3307 | replicator/password123 |
| PostgreSQL Target | localhost:5433 | replicator/password123 |
| Elasticsearch | http://localhost:9200 | - |
| **Messaging** |
| Kafka | localhost:9092 | - |
| Redis | localhost:6379 | password123 |

## 💾 Working with Data

### Inspect Source Data

```bash
# MongoDB
docker exec replicator-mongodb-source mongosh \
  "mongodb://admin:password123@localhost:27017/source_db"

# MySQL
docker exec -it replicator-mysql-source mysql \
  -u replicator -ppassword123 source_db

# PostgreSQL
docker exec -it replicator-postgresql-source psql \
  -U replicator -d source_db
```

### Check Replicated Data

```bash
# MongoDB Target
docker exec replicator-mongodb-target mongosh \
  "mongodb://admin:password123@localhost:27017/target_db"

# Elasticsearch
curl "http://localhost:9200/products/_search?pretty"

# Kafka Topics
docker exec replicator-kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic orders-stream --from-beginning
```

### Insert Test Data

```bash
# Insert into MongoDB source
docker exec replicator-mongodb-source mongosh --eval "
  db = db.getSiblingDB('source_db');
  db.users.insertOne({
    name: 'New User',
    email: 'new.user@example.com',
    department: 'Engineering'
  });
"

# Insert into MySQL source
docker exec replicator-mysql-source mysql \
  -u replicator -ppassword123 -D source_db -e "
  INSERT INTO products (name, description, price, category_id) 
  VALUES ('New Product', 'Test product', 199.99, 1);
"

# Insert into PostgreSQL source
docker exec replicator-postgresql-source psql \
  -U replicator -d source_db -c "
  INSERT INTO orders (customer_id, total_amount, status) 
  VALUES (123, 299.99, 'pending');
"
```

## 🔍 Monitoring and Debugging

### Check Application Health

```bash
# Quick health check
curl http://localhost:8080/health | jq

# Detailed metrics
curl http://localhost:9090/metrics | grep replicator

# Stream status
curl http://localhost:8080/streams | jq
```

### Monitor Logs

```bash
# All services
docker-compose logs -f

# Specific service
./quickstart.sh logs replicator
./quickstart.sh logs mongodb-source
./quickstart.sh logs elasticsearch

# Filter for errors
docker-compose logs replicator | grep ERROR
```

### Performance Monitoring

Access Grafana at http://localhost:3000 (admin/admin123) for:
- **Events Processed**: Real-time replication throughput
- **Processing Latency**: End-to-end processing time
- **Error Rates**: Failed replication attempts
- **Resource Usage**: CPU, memory, disk utilization

## 🛠️ Customization

### Create Custom Configuration

1. Copy an existing config:
```bash
cp examples/configs/mongodb-to-mongodb.yaml examples/configs/my-config.yaml
```

2. Modify connection strings, transformations, or error handling

3. Run with custom config:
```bash
./quickstart.sh run my-config
```

### Add New Databases

1. Edit `docker-compose.yml` to add your database service
2. Create initialization scripts in `examples/init-scripts/`
3. Create configuration file in `examples/configs/`
4. Test with `./quickstart.sh run your-config`

### Modify Transformations

Edit the transformation rules in configuration files:

```yaml
transforms:
  - type: "field-mapping"
    config:
      mapping:
        "source_field": "$.target_field"
        "computed_field": "concat($.field1, ' ', $.field2)"
  - type: "field-enrichment"
    config:
      fields:
        "enriched_data": "your_custom_logic_here"
```

## 🧪 Testing Scenarios

### Automated Testing

```bash
# Test specific scenario
./quickstart.sh test mongodb-to-mongodb

# Test all scenarios
for scenario in mongodb-to-mongodb mysql-to-elasticsearch postgresql-to-kafka multi-source-aggregation; do
  echo "Testing $scenario..."
  ./quickstart.sh run $scenario
  sleep 30
  ./quickstart.sh test $scenario
  ./quickstart.sh stop
done
```

### Manual Verification

1. **Data Consistency**: Compare record counts between source and target
2. **Transformation Accuracy**: Verify field mappings and computed values
3. **Error Handling**: Intentionally insert invalid data and check error logs
4. **Performance**: Monitor processing latency under different load conditions

## 🚀 Next Steps

### Local Development
1. **Explore Configuration Options**: Review all YAML files in `examples/configs/`
2. **Experiment with Transformations**: Modify field mappings and enrichment rules
3. **Test Error Scenarios**: Disconnect databases, insert invalid data
4. **Monitor Performance**: Use Grafana dashboards to understand system behavior

### Production Deployment
1. **Review Azure Configuration**: See [Azure Setup Guide](#azure-configuration)
2. **Kubernetes Deployment**: Use Helm charts in `deploy/helm/`
3. **Security Hardening**: Implement proper authentication and encryption
4. **Backup Strategy**: Plan for position tracking and disaster recovery

### Advanced Features
1. **Custom Transformations**: Implement custom transformation logic
2. **Webhook Integration**: Add real-time notifications
3. **Multi-tenancy**: Configure isolated streams per tenant
4. **Auto-scaling**: Implement dynamic scaling based on load

## 🧹 Cleanup

```bash
# Stop all services
./quickstart.sh cleanup

# This will:
# - Stop all containers
# - Remove all volumes (data will be lost)
# - Clean up Docker networks
```

## 🆘 Troubleshooting

### Common Issues

**Services won't start**:
```bash
# Check port conflicts
./quickstart.sh prereq

# Check Docker resources
docker system df
docker system prune
```

**Replication not working**:
```bash
# Check service health
./quickstart.sh services

# Verify database connections
docker-compose ps

# Check logs for errors
./quickstart.sh logs replicator
```

**Performance issues**:
- Reduce batch sizes in configuration files
- Check available system resources
- Monitor Grafana dashboards for bottlenecks

### Getting Help

1. **Check Application Logs**: `./quickstart.sh logs`
2. **Verify Service Health**: `curl http://localhost:8080/health`
3. **Review Configuration**: Validate YAML syntax and connection strings
4. **Monitor Resources**: Check Docker stats and system resources

## Azure Configuration

For production Azure deployment with Entra authentication:

1. **Create Azure Resources**: CosmosDB, Event Hubs, etc.
2. **Set up Entra Authentication**: Service principal with appropriate permissions
3. **Use Azure Configuration Templates**: See `examples/configs/azure/` (after setup)
4. **Deploy with Helm**: Use production-ready Helm charts in `deploy/helm/`

**Note**: Azure configurations are excluded from this repository for security. Contact your Azure administrator for environment-specific settings.