-- MySQL Source Database Initialization
-- Creates sample tables and data for replication testing

USE source_db;

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    age INT,
    department VARCHAR(50),
    salary DECIMAL(10,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    status ENUM('active', 'inactive', 'pending') DEFAULT 'active',
    INDEX idx_department (department),
    INDEX idx_email (email),
    INDEX idx_created_at (created_at)
);

-- Create products table
CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    price DECIMAL(10,2) NOT NULL,
    category_id INT,
    brand VARCHAR(100),
    in_stock BOOLEAN DEFAULT TRUE,
    quantity INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_category (category_id),
    INDEX idx_price (price),
    INDEX idx_name (name),
    FULLTEXT idx_search (name, description)
);

-- Create orders table
CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    customer_id INT NOT NULL,
    total_amount DECIMAL(12,2) NOT NULL,
    status ENUM('pending', 'processing', 'shipped', 'delivered', 'cancelled') DEFAULT 'pending',
    order_date DATE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_customer (customer_id),
    INDEX idx_status (status),
    INDEX idx_order_date (order_date)
);

-- Create order_items table
CREATE TABLE IF NOT EXISTS order_items (
    id INT AUTO_INCREMENT PRIMARY KEY,
    order_id INT NOT NULL,
    product_id INT NOT NULL,
    quantity INT NOT NULL,
    unit_price DECIMAL(10,2) NOT NULL,
    total_price DECIMAL(12,2) GENERATED ALWAYS AS (quantity * unit_price) STORED,
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    INDEX idx_order (order_id),
    INDEX idx_product (product_id)
);

-- Insert sample data
INSERT INTO users (name, email, age, department, salary) VALUES
('John Doe', 'john.doe@example.com', 30, 'Engineering', 75000.00),
('Jane Smith', 'jane.smith@example.com', 28, 'Marketing', 65000.00),
('Bob Johnson', 'bob.johnson@example.com', 35, 'Sales', 80000.00),
('Alice Brown', 'alice.brown@example.com', 32, 'Engineering', 85000.00),
('Charlie Wilson', 'charlie.wilson@example.com', 29, 'Marketing', 62000.00);

INSERT INTO products (name, description, price, category_id, brand, quantity) VALUES
('Laptop Pro', 'High-performance laptop for professionals', 1299.99, 1, 'TechCorp', 50),
('Wireless Mouse', 'Ergonomic wireless mouse with precision tracking', 29.99, 2, 'TechCorp', 200),
('Mechanical Keyboard', 'RGB mechanical keyboard for gaming and productivity', 149.99, 2, 'GameTech', 75),
('Monitor 4K', '27-inch 4K UHD monitor with HDR support', 399.99, 1, 'DisplayMax', 30),
('Webcam HD', 'Full HD webcam with auto-focus', 79.99, 2, 'VisionTech', 100);

INSERT INTO orders (customer_id, total_amount, status, order_date) VALUES
(1, 1329.98, 'completed', CURDATE()),
(2, 229.98, 'shipped', CURDATE() - INTERVAL 1 DAY),
(3, 79.99, 'processing', CURDATE()),
(1, 549.98, 'delivered', CURDATE() - INTERVAL 3 DAY),
(4, 1699.97, 'pending', CURDATE());

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

-- Enable binary logging for replication
-- Note: These settings should be in my.cnf, but including here for reference
-- SET GLOBAL binlog_format = 'ROW';
-- SET GLOBAL log_bin = ON;

SELECT 'MySQL source database initialization completed!' as status;
SELECT COUNT(*) as user_count FROM users;
SELECT COUNT(*) as product_count FROM products;
SELECT COUNT(*) as order_count FROM orders;