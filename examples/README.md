# Replicator Examples

This directory contains complete examples and configurations for testing Replicator in various scenarios, from local development to production Azure deployments.

## 📁 Directory Structure

```
examples/
├── configs/                    # Configuration files for different scenarios
│   ├── mongodb-to-mongodb.yaml     # Basic MongoDB replication
│   ├── mysql-to-elasticsearch.yaml # MySQL to search index
│   ├── postgresql-to-kafka.yaml    # Change data capture to Kafka
│   ├── multi-source-aggregation.yaml # Multi-source analytics
│   └── azure/                      # Azure-specific configurations
│       ├── README.md                   # Azure setup guide
│       ├── cosmos-to-eventhub.yaml.template
│       └── sql-to-cosmos.yaml.template
├── init-scripts/               # Database initialization scripts
│   ├── mongo-source-init.js        # MongoDB sample data
│   ├── mongo-target-init.js        # MongoDB target setup
│   ├── mysql-source-init.sql       # MySQL sample tables and data
│   ├── mysql-target-init.sql       # MySQL target setup
│   ├── postgresql-source-init.sql  # PostgreSQL sample data
│   └── postgresql-target-init.sql  # PostgreSQL target setup
├── positions/                  # Position tracking storage (local)
└── README.md                   # This file
```

## 🚀 Quick Start

### Prerequisites
- Docker and Docker Compose
- 8GB+ RAM available
- Ports 3000, 5432-5433, 6379, 8080, 9090-9092, 9200, 27017-27018 available

### 1. Start Complete Environment

```bash
# From project root
./quickstart.sh start
```

This starts:
- **Source Databases**: MongoDB, MySQL, PostgreSQL with sample data
- **Target Systems**: MongoDB, MySQL, PostgreSQL, Elasticsearch
- **Messaging**: Kafka with Zookeeper, Redis
- **Monitoring**: Prometheus, Grafana with dashboards
- **Application**: Replicator (ready for configuration)

### 2. Run Example Scenarios

```bash
# MongoDB to MongoDB replication
./quickstart.sh run mongodb-to-mongodb
./quickstart.sh test mongodb-to-mongodb

# MySQL to Elasticsearch indexing  
./quickstart.sh run mysql-to-elasticsearch
./quickstart.sh test mysql-to-elasticsearch

# PostgreSQL to Kafka streaming
./quickstart.sh run postgresql-to-kafka
./quickstart.sh test postgresql-to-kafka

# Multi-source aggregation
./quickstart.sh run multi-source-aggregation
./quickstart.sh test multi-source-aggregation
```

## 📋 Example Scenarios

### 1. MongoDB to MongoDB (`mongodb-to-mongodb.yaml`)

**Use Case**: Basic document replication between MongoDB instances
- **Source**: MongoDB with change streams
- **Target**: MongoDB with field mapping
- **Features**: Real-time replication, field transformation, position tracking

**What you'll learn**:
- Change stream processing
- Document field mapping and renaming
- Error handling with retries
- Position-based resume capability

**Test it**:
```bash
# Start the scenario
./quickstart.sh run mongodb-to-mongodb

# Insert test data
docker exec replicator-mongodb-source mongosh --eval "
  db = db.getSiblingDB('source_db');
  db.users.insertOne({
    name: 'Test User',
    email: 'test@example.com',
    department: 'Engineering'
  });"

# Check replicated data
docker exec replicator-mongodb-target mongosh --eval "
  db = db.getSiblingDB('target_db');
  db.users.find().pretty();"
```

### 2. MySQL to Elasticsearch (`mysql-to-elasticsearch.yaml`)

**Use Case**: Real-time search indexing from transactional database
- **Source**: MySQL with binary log processing
- **Target**: Elasticsearch with bulk indexing
- **Features**: Data enrichment, search optimization, bulk operations

**What you'll learn**:
- Binary log (binlog) change data capture
- Data transformation and enrichment
- Elasticsearch bulk indexing patterns
- Search keyword generation

**Test it**:
```bash
# Start the scenario
./quickstart.sh run mysql-to-elasticsearch

# Insert product data
docker exec replicator-mysql-source mysql -u replicator -ppassword123 -D source_db -e "
  INSERT INTO products (name, description, price, category_id) 
  VALUES ('Amazing Product', 'High-quality amazing product', 199.99, 1);"

# Search in Elasticsearch
curl "http://localhost:9200/products/_search?q=amazing&pretty"
```

### 3. PostgreSQL to Kafka (`postgresql-to-kafka.yaml`)

**Use Case**: Change data capture for event-driven architecture
- **Source**: PostgreSQL with logical replication
- **Target**: Kafka topics with partitioning
- **Features**: Change event enveloping, ordered processing, dead letter queues

**What you'll learn**:
- PostgreSQL logical replication slots
- Debezium-style change event format
- Kafka partitioning strategies
- Event ordering guarantees

**Test it**:
```bash
# Start the scenario
./quickstart.sh run postgresql-to-kafka

# Insert order data
docker exec replicator-postgresql-source psql -U replicator -d source_db -c "
  INSERT INTO orders (customer_id, total_amount, status) 
  VALUES (123, 299.99, 'pending');"

# Consume from Kafka
docker exec replicator-kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic orders-stream --from-beginning
```

### 4. Multi-Source Aggregation (`multi-source-aggregation.yaml`)

**Use Case**: Combining data from multiple sources for analytics
- **Sources**: MongoDB users, MySQL orders, PostgreSQL products
- **Target**: Elasticsearch analytics index
- **Features**: Multiple concurrent streams, data correlation, unified view

**What you'll learn**:
- Managing multiple source connections
- Data aggregation patterns
- Stream coordination and monitoring
- Analytics data modeling

**Test it**:
```bash
# Start the scenario
./quickstart.sh run multi-source-aggregation

# The test script will insert data in all sources
./quickstart.sh test multi-source-aggregation

# View aggregated analytics
curl "http://localhost:9200/customer-analytics/_search?pretty"
```

## 🛠️ Customizing Examples

### Modify Transformations

Edit the transformation rules in any configuration file:

```yaml
transforms:
  - type: "field-mapping"
    config:
      mapping:
        "new_field": "$.source_field"
        "computed": "concat($.field1, ' - ', $.field2)"
        "enriched": "upper($.text_field)"
        
  - type: "field-enrichment"
    config:
      fields:
        "category": |
          if $.price < 50 then "budget"
          elif $.price < 200 then "mid-range"
          else "premium"
          end
```

### Add New Databases

1. **Add to docker-compose.yml**:
```yaml
my-database:
  image: postgres:15
  environment:
    POSTGRES_DB: my_db
    POSTGRES_USER: user
    POSTGRES_PASSWORD: pass
  ports:
    - "5434:5432"
```

2. **Create initialization script** in `init-scripts/my-database-init.sql`

3. **Create configuration** in `configs/my-scenario.yaml`

4. **Test** with `./quickstart.sh run my-scenario`

### Create Custom Scenarios

Copy an existing configuration and modify:

```bash
# Copy base configuration
cp examples/configs/mongodb-to-mongodb.yaml examples/configs/my-scenario.yaml

# Edit connection strings, transformations, error handling
nano examples/configs/my-scenario.yaml

# Test your scenario
./quickstart.sh run my-scenario
```

## 🔍 Monitoring Examples

### Application Metrics

```bash
# Check health
curl http://localhost:8080/health | jq

# View metrics
curl http://localhost:9090/metrics | grep replicator

# Stream details
curl http://localhost:8080/streams | jq
```

### Grafana Dashboards

Access Grafana at http://localhost:3000 (admin/admin123):

1. **Replicator Overview**: General system health and throughput
2. **Stream Performance**: Per-stream metrics and error rates  
3. **Resource Usage**: CPU, memory, and storage utilization
4. **Error Analysis**: Error patterns and troubleshooting

### Log Analysis

```bash
# Application logs
./quickstart.sh logs replicator

# Database logs
./quickstart.sh logs mongodb-source
./quickstart.sh logs mysql-source
./quickstart.sh logs postgresql-source

# Filter for errors
docker-compose logs replicator | grep ERROR
```

## 🐛 Troubleshooting

### Common Issues

**Services won't start**:
```bash
# Check prerequisites
./quickstart.sh prereq

# Check system resources
docker system df
docker stats

# Clear old containers/volumes
docker-compose down -v
docker system prune
```

**Replication not working**:
```bash
# Verify service health
curl http://localhost:8080/health

# Check database connectivity
docker exec replicator-mongodb-source mongosh --eval "db.runCommand('ping')"

# Review configuration
./quickstart.sh logs replicator | grep -i error
```

**Data not appearing in target**:
```bash
# Check stream status
curl http://localhost:8080/streams/STREAM_NAME

# Verify source has data
docker exec replicator-mongodb-source mongosh --eval "
  db = db.getSiblingDB('source_db');
  db.users.countDocuments();"

# Check target connectivity
curl http://localhost:9200/_cluster/health
```

### Performance Issues

1. **High memory usage**: Reduce batch sizes in configuration
2. **Slow processing**: Check transformation complexity
3. **Network timeouts**: Increase timeout values
4. **Disk space**: Monitor Docker volume usage

## 🚀 Next Steps

### Local Development
1. **Experiment** with different transformation rules
2. **Test error scenarios** by disconnecting services
3. **Monitor performance** under different loads
4. **Create custom scenarios** for your use cases

### Production Deployment
1. **Review Azure templates** in `configs/azure/`
2. **Deploy with Kubernetes** using Helm charts
3. **Implement proper security** and authentication
4. **Set up monitoring** and alerting

### Advanced Features
1. **Custom transformations** with complex business logic
2. **Multi-tenancy** with isolated streams
3. **Auto-scaling** based on workload
4. **Disaster recovery** and backup strategies

## 📚 Additional Resources

- **Configuration Reference**: See individual YAML files for detailed options
- **Azure Setup**: Check `configs/azure/README.md` for cloud deployment
- **Helm Charts**: Use `deploy/helm/` for Kubernetes deployment
- **API Documentation**: View OpenAPI spec for programmatic control

For questions or issues, check the logs first, then refer to the main project documentation.