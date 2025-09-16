#!/usr/bin/env node

/**
 * Data Transformation Script for MySQL Products Table to Elasticsearch
 * 
 * This script demonstrates how to transform MySQL row data to properly named
 * Elasticsearch documents. In a production system, this mapping would be
 * configurable through a schema file or configuration.
 */

const mysql_products_schema = [
  { index: 0, mysql_field: 'id', es_field: 'id', type: 'keyword' },
  { index: 1, mysql_field: 'name', es_field: 'name', type: 'text' },
  { index: 2, mysql_field: 'description', es_field: 'description', type: 'text' },
  { index: 3, mysql_field: 'price', es_field: 'price', type: 'double' },
  { index: 4, mysql_field: 'category_id', es_field: 'category_id', type: 'integer' },
  { index: 5, mysql_field: 'brand', es_field: 'brand', type: 'text' },
  { index: 6, mysql_field: 'in_stock', es_field: 'in_stock', type: 'boolean' },
  { index: 7, mysql_field: 'quantity', es_field: 'quantity', type: 'integer' },
  { index: 8, mysql_field: 'created_at', es_field: 'created_at', type: 'date' },
  { index: 9, mysql_field: 'updated_at', es_field: 'updated_at', type: 'date' }
];

/**
 * Transform MySQL row array to Elasticsearch document
 * @param {Array} rowData - Array of values from MySQL binlog
 * @returns {Object} - Transformed document for Elasticsearch
 */
function transformProductsRow(rowData) {
  const document = {};
  
  mysql_products_schema.forEach(field => {
    if (rowData.length > field.index) {
      document[field.es_field] = rowData[field.index];
    }
  });
  
  return document;
}

/**
 * Generate Elasticsearch index mapping for products
 * @returns {Object} - Index mapping configuration
 */
function generateProductsMapping() {
  const properties = {};
  
  mysql_products_schema.forEach(field => {
    switch (field.type) {
      case 'text':
        properties[field.es_field] = {
          type: 'text',
          fields: {
            keyword: {
              type: 'keyword'
            }
          }
        };
        break;
      case 'keyword':
        properties[field.es_field] = { type: 'keyword' };
        break;
      case 'double':
        properties[field.es_field] = { type: 'double' };
        break;
      case 'integer':
        properties[field.es_field] = { type: 'integer' };
        break;
      case 'boolean':
        properties[field.es_field] = { type: 'boolean' };
        break;
      case 'date':
        properties[field.es_field] = { type: 'date' };
        break;
      default:
        properties[field.es_field] = { type: 'text' };
    }
  });
  
  return {
    settings: {
      number_of_shards: 1,
      number_of_replicas: 0
    },
    mappings: {
      properties: properties
    }
  };
}

// Example usage
if (require.main === module) {
  // Example MySQL row data
  const exampleRow = [
    1,                    // id
    'iPhone 15',          // name
    'Latest iPhone',      // description
    999.99,              // price
    1,                   // category_id
    'Apple',             // brand
    true,                // in_stock
    50,                  // quantity
    '2023-09-12T10:00:00Z', // created_at
    '2023-09-12T10:00:00Z'  // updated_at
  ];
  
  console.log('Example transformation:');
  console.log('Input (MySQL row):', exampleRow);
  console.log('Output (ES document):', transformProductsRow(exampleRow));
  console.log('\nElasticsearch mapping:');
  console.log(JSON.stringify(generateProductsMapping(), null, 2));
}

module.exports = {
  transformProductsRow,
  generateProductsMapping,
  mysql_products_schema
};