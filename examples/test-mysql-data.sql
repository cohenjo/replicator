-- Test data for MySQL replication
USE source_db;

-- Insert some test products
INSERT INTO products (name, description, price, category_id, brand, in_stock, quantity) VALUES
('Laptop', 'High-performance laptop for developers', 1299.99, 1, 'TechBrand', TRUE, 50),
('Wireless Mouse', 'Ergonomic wireless mouse with long battery life', 29.99, 2, 'MouseCorp', TRUE, 200),
('Mechanical Keyboard', 'RGB backlit mechanical keyboard', 149.99, 2, 'KeyTech', TRUE, 75),
('Monitor', '27-inch 4K display monitor', 399.99, 1, 'ScreenPro', TRUE, 30),
('Smartphone', 'Latest flagship smartphone', 899.99, 3, 'PhoneMaker', TRUE, 100);

-- Update a product to trigger an update event
UPDATE products SET price = 1199.99, quantity = 45 WHERE name = 'Laptop';

-- Delete a product to trigger a delete event  
DELETE FROM products WHERE name = 'Wireless Mouse';

-- Insert more products
INSERT INTO products (name, description, price, category_id, brand, in_stock, quantity) VALUES
('Tablet', '10-inch tablet for content consumption', 299.99, 3, 'TabletMaker', TRUE, 80),
('Headphones', 'Noise-cancelling wireless headphones', 199.99, 4, 'AudioTech', TRUE, 120);