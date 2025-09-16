-- PostgreSQL Target Database Initialization
-- Prepares target database for receiving replicated data

-- Create users table for replicated data
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(50) PRIMARY KEY,
    full_name VARCHAR(100),
    email_address VARCHAR(100),
    timestamp TIMESTAMP,
    user_status VARCHAR(20),
    department VARCHAR(50),
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create analytics table for aggregated data
CREATE TABLE IF NOT EXISTS customer_analytics (
    id SERIAL PRIMARY KEY,
    document_type VARCHAR(50),
    user_id VARCHAR(50),
    customer_id VARCHAR(50),
    order_id VARCHAR(50),
    product_id VARCHAR(50),
    data JSONB,
    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create change events table for Kafka-style events
CREATE TABLE IF NOT EXISTS change_events (
    id SERIAL PRIMARY KEY,
    source_table VARCHAR(100),
    operation VARCHAR(10), -- INSERT, UPDATE, DELETE
    before_data JSONB,
    after_data JSONB,
    transaction_id BIGINT,
    timestamp_ms BIGINT,
    source_metadata JSONB,
    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create replication metadata table
CREATE TABLE IF NOT EXISTS replication_metadata (
    source_name VARCHAR(100) PRIMARY KEY,
    last_processed_lsn VARCHAR(50),
    last_processed_timestamp TIMESTAMP,
    records_processed INTEGER DEFAULT 0,
    last_error TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email_address);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(user_status);
CREATE INDEX IF NOT EXISTS idx_users_timestamp ON users(timestamp);

CREATE INDEX IF NOT EXISTS idx_analytics_type ON customer_analytics(document_type);
CREATE INDEX IF NOT EXISTS idx_analytics_user ON customer_analytics(user_id);
CREATE INDEX IF NOT EXISTS idx_analytics_customer ON customer_analytics(customer_id);
CREATE INDEX IF NOT EXISTS idx_analytics_processed ON customer_analytics(processed_at);

CREATE INDEX IF NOT EXISTS idx_events_table ON change_events(source_table);
CREATE INDEX IF NOT EXISTS idx_events_operation ON change_events(operation);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON change_events(timestamp_ms);
CREATE INDEX IF NOT EXISTS idx_events_processed ON change_events(processed_at);

-- Create function to update metadata timestamp
CREATE OR REPLACE FUNCTION update_replication_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger for metadata updates
CREATE TRIGGER update_replication_metadata_timestamp 
    BEFORE UPDATE ON replication_metadata
    FOR EACH ROW EXECUTE FUNCTION update_replication_timestamp();

SELECT 'PostgreSQL target database initialization completed!' as status;
SELECT 'Ready to receive replicated data from various sources.' as message;