#!/bin/bash

# Setup Products Index in Elasticsearch
# This script creates the proper index mapping for the MySQL products table

ES_HOST="${ES_HOST:-localhost:9200}"
INDEX_NAME="${INDEX_NAME:-products}"

echo "Setting up Elasticsearch index: $INDEX_NAME"
echo "Elasticsearch host: $ES_HOST"

# Delete existing index if it exists
echo "Deleting existing index (if exists)..."
curl -X DELETE "http://$ES_HOST/$INDEX_NAME" 2>/dev/null || true

# Create index with proper mapping for products table
echo "Creating index with proper mapping..."
curl -X PUT "http://$ES_HOST/$INDEX_NAME" \
  -H "Content-Type: application/json" \
  -d '{
    "settings": {
      "number_of_shards": 1,
      "number_of_replicas": 0
    },
    "mappings": {
      "properties": {
        "id": {
          "type": "keyword"
        },
        "name": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword"
            }
          }
        },
        "description": {
          "type": "text"
        },
        "price": {
          "type": "double"
        },
        "category_id": {
          "type": "integer"
        },
        "brand": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword"
            }
          }
        },
        "in_stock": {
          "type": "boolean"
        },
        "quantity": {
          "type": "integer"
        },
        "created_at": {
          "type": "date"
        },
        "updated_at": {
          "type": "date"
        }
      }
    }
  }'

echo ""
echo "Index setup complete!"

# Verify the index was created
echo "Verifying index creation..."
curl -s "http://$ES_HOST/_cat/indices/$INDEX_NAME?v"