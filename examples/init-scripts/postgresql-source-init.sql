-- PostgreSQL Source Database Initialization
-- Creates sample tables and data for replication testing

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    age INTEGER,
    department VARCHAR(50),
    salary DECIMAL(10,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) DEFAULT 'active',
    metadata JSONB
);

-- Create products table
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    price DECIMAL(10,2) NOT NULL,
    category_id INTEGER,
    brand VARCHAR(100),
    in_stock BOOLEAN DEFAULT TRUE,
    quantity INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    attributes JSONB
);

-- Create orders table
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL,
    total_amount DECIMAL(12,2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    order_date DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    order_data JSONB
);

-- Create order_items table
CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES orders(id) ON DELETE CASCADE,
    product_id INTEGER NOT NULL,
    quantity INTEGER NOT NULL,
    unit_price DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_department ON users(department);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);

CREATE INDEX IF NOT EXISTS idx_products_category ON products(category_id);
CREATE INDEX IF NOT EXISTS idx_products_price ON products(price);
CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);

CREATE INDEX IF NOT EXISTS idx_orders_customer ON orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_date ON orders(order_date);

CREATE INDEX IF NOT EXISTS idx_order_items_order ON order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_order_items_product ON order_items(product_id);

-- Insert sample data
INSERT INTO users (name, email, age, department, salary, metadata) VALUES
('John Doe', 'john.doe@example.com', 30, 'Engineering', 75000.00, '{"skills": ["Go", "PostgreSQL", "Docker"], "location": "San Francisco"}'),
('Jane Smith', 'jane.smith@example.com', 28, 'Marketing', 65000.00, '{"skills": ["Marketing", "Analytics"], "location": "New York"}'),
('Bob Johnson', 'bob.johnson@example.com', 35, 'Sales', 80000.00, '{"skills": ["Sales", "CRM"], "location": "Chicago"}'),
('Alice Brown', 'alice.brown@example.com', 32, 'Engineering', 85000.00, '{"skills": ["Python", "Machine Learning"], "location": "Seattle"}'),
('Charlie Wilson', 'charlie.wilson@example.com', 29, 'Marketing', 62000.00, '{"skills": ["Content", "SEO"], "location": "Austin"}');

INSERT INTO products (name, description, price, category_id, brand, quantity, attributes) VALUES
('Laptop Pro', 'High-performance laptop for professionals', 1299.99, 1, 'TechCorp', 50, '{"processor": "Intel i7", "ram": "16GB", "storage": "512GB SSD"}'),
('Wireless Mouse', 'Ergonomic wireless mouse with precision tracking', 29.99, 2, 'TechCorp', 200, '{"wireless": true, "battery_life": "6 months", "dpi": 1600}'),
('Mechanical Keyboard', 'RGB mechanical keyboard for gaming and productivity', 149.99, 2, 'GameTech', 75, '{"switches": "Cherry MX Blue", "backlight": "RGB", "layout": "US"}'),
('Monitor 4K', '27-inch 4K UHD monitor with HDR support', 399.99, 1, 'DisplayMax', 30, '{"size": "27 inch", "resolution": "4K", "refresh_rate": "60Hz"}'),
('Webcam HD', 'Full HD webcam with auto-focus', 79.99, 2, 'VisionTech', 100, '{"resolution": "1080p", "autofocus": true, "microphone": true}');

INSERT INTO orders (customer_id, total_amount, status, order_date, order_data) VALUES
(1, 1329.98, 'completed', CURRENT_DATE, '{"payment_method": "credit_card", "shipping": "express"}'),
(2, 229.98, 'shipped', CURRENT_DATE - INTERVAL '1 day', '{"payment_method": "paypal", "shipping": "standard"}'),
(3, 79.99, 'processing', CURRENT_DATE, '{"payment_method": "credit_card", "shipping": "standard"}'),
(1, 549.98, 'delivered', CURRENT_DATE - INTERVAL '3 days', '{"payment_method": "credit_card", "shipping": "express"}'),
(4, 1699.97, 'pending', CURRENT_DATE, '{"payment_method": "bank_transfer", "shipping": "express"}');

INSERT INTO order_items (order_id, product_id, quantity, unit_price) VALUES
(1, 1, 1, 1299.99),
(1, 2, 1, 29.99),
(2, 3, 1, 149.99),
(2, 5, 1, 79.99),
(3, 5, 1, 79.99),
(4, 1, 1, 1299.99),
(4, 4, 1, 399.99),
(5, 1, 1, 1299.99),
(5, 4, 1, 399.99);

-- Create publication for logical replication
CREATE PUBLICATION replicator_orders_pub FOR TABLE orders;
CREATE PUBLICATION replicator_products_pub FOR TABLE products;
CREATE PUBLICATION replicator_users_pub FOR TABLE users;

-- Create replication slot (will be used by replicator)
-- SELECT pg_create_logical_replication_slot('replicator_orders_slot', 'pgoutput');
-- SELECT pg_create_logical_replication_slot('replicator_products_slot', 'pgoutput');
-- SELECT pg_create_logical_replication_slot('replicator_users_slot', 'pgoutput');

-- Update function for updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_orders_updated_at BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

SELECT 'PostgreSQL source database initialization completed!' as status;
SELECT COUNT(*) as user_count FROM users;
SELECT COUNT(*) as product_count FROM products;
SELECT COUNT(*) as order_count FROM orders;