# Elasticsearch Setup for Replicator

This directory contains scripts and configurations for setting up Elasticsearch indices and schema mappings for the replicator demo.

## Files

### setup-products-index.sh
Bash script that creates the Elasticsearch index with proper mapping for the MySQL products table.

**Usage:**
```bash
# Use default settings (localhost:9200, index name 'products')
./setup-products-index.sh

# Use custom Elasticsearch host
ES_HOST=my-elasticsearch:9200 ./setup-products-index.sh

# Use custom index name
INDEX_NAME=my_products ./setup-products-index.sh
```

### products-schema-mapping.js
Node.js script that demonstrates how to transform MySQL row data to properly named Elasticsearch documents. This shows the schema mapping that should be implemented in the replicator for production use.

**Usage:**
```bash
# Run the example
node products-schema-mapping.js
```

## Production Considerations

In a production system, the schema mapping should be:

1. **Configurable**: Schema mappings should be defined in configuration files, not hardcoded
2. **Dynamic**: Support for multiple table schemas and transformations
3. **Validated**: Proper type checking and validation during transformation
4. **Performant**: Efficient transformation without blocking the replication pipeline

## Current Implementation

The current replicator implementation uses a generic approach where:
- MySQL row arrays are converted to `field_0`, `field_1`, etc.
- Elasticsearch uses dynamic mapping to auto-detect field types
- The first column is used as the document ID

For demo purposes, run the setup script first:
```bash
./examples/elasticsearch/setup-products-index.sh
```

Then start the replicator:
```bash
./replicator --config ./examples/configs/mysql-to-elasticsearch-new.yaml
```