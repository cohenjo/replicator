-- MySQL Target Database Initialization
-- Prepares target database for receiving replicated data

USE target_db;

-- Create users table for replicated data
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(50) PRIMARY KEY,
    full_name VARCHAR(100),
    email_address VARCHAR(100),
    timestamp TIMESTAMP,
    user_status VARCHAR(20),
    department VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_email (email_address),
    INDEX idx_status (user_status),
    INDEX idx_timestamp (timestamp)
);

-- Create products table for Elasticsearch indexing
CREATE TABLE IF NOT EXISTS products_search (
    product_id VARCHAR(50) PRIMARY KEY,
    title VARCHAR(200),
    description TEXT,
    price DECIMAL(10,2),
    category VARCHAR(100),
    search_keywords TEXT,
    price_range VARCHAR(20),
    indexed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_category (category),
    INDEX idx_price_range (price_range),
    FULLTEXT idx_search (title, description, search_keywords)
);

-- Create analytics summary table
CREATE TABLE IF NOT EXISTS customer_analytics (
    id INT AUTO_INCREMENT PRIMARY KEY,
    document_type VARCHAR(50),
    entity_id VARCHAR(50),
    data JSON,
    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_type (document_type),
    INDEX idx_entity (entity_id),
    INDEX idx_processed (processed_at)
);

-- Create replication status tracking table
CREATE TABLE IF NOT EXISTS replication_status (
    source_name VARCHAR(100) PRIMARY KEY,
    last_processed_id VARCHAR(100),
    last_processed_timestamp TIMESTAMP,
    records_processed INT DEFAULT 0,
    last_error TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

SELECT 'MySQL target database initialization completed!' as status;
SELECT 'Ready to receive replicated data from various sources.' as message;