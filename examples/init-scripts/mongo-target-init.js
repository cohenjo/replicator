// MongoDB Target Database Initialization
// Prepares target database for receiving replicated data

print('Starting MongoDB target database initialization...');

// Switch to target database
db = db.getSiblingDB('target_db');

// Create collections that will receive replicated data
db.createCollection('users');
db.createCollection('products');
db.createCollection('orders');
db.createCollection('customer_analytics');

// Create indexes for replicated data
db.users.createIndex({ "email": 1 });
db.users.createIndex({ "full_name": 1 });
db.users.createIndex({ "timestamp": 1 });
db.users.createIndex({ "user_status": 1 });

db.products.createIndex({ "category": 1 });
db.products.createIndex({ "price": 1 });
db.products.createIndex({ "indexed_at": 1 });

db.orders.createIndex({ "customer_id": 1 });
db.orders.createIndex({ "order_date": 1 });
db.orders.createIndex({ "status": 1 });

db.customer_analytics.createIndex({ "document_type": 1 });
db.customer_analytics.createIndex({ "user_id": 1 });
db.customer_analytics.createIndex({ "customer_id": 1 });

print('MongoDB target database initialization completed successfully!');
print('Target collections created with appropriate indexes.');
print('Ready to receive replicated data from various sources.');