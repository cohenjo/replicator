// MongoDB Source Database Initialization
// Creates sample collections and data for replication testing

print('Starting MongoDB source database initialization...');

// Switch to source database
db = db.getSiblingDB('source_db');

// Create users collection with sample data
db.users.insertMany([
  {
    _id: ObjectId(),
    name: "John Doe",
    email: "john.doe@example.com",
    age: 30,
    department: "Engineering",
    salary: 75000,
    created_at: new Date(),
    updated_at: new Date(),
    status: "active",
    address: {
      street: "123 Main St",
      city: "San Francisco",
      state: "CA",
      zip: "94105"
    },
    skills: ["JavaScript", "Go", "MongoDB"]
  },
  {
    _id: ObjectId(),
    name: "Jane Smith",
    email: "jane.smith@example.com",
    age: 28,
    department: "Marketing",
    salary: 65000,
    created_at: new Date(),
    updated_at: new Date(),
    status: "active",
    address: {
      street: "456 Oak Ave",
      city: "New York",
      state: "NY",
      zip: "10001"
    },
    skills: ["Marketing", "Analytics", "Content"]
  },
  {
    _id: ObjectId(),
    name: "Bob Johnson",
    email: "bob.johnson@example.com",
    age: 35,
    department: "Sales",
    salary: 80000,
    created_at: new Date(),
    updated_at: new Date(),
    status: "active",
    address: {
      street: "789 Pine Rd",
      city: "Chicago",
      state: "IL",
      zip: "60601"
    },
    skills: ["Sales", "CRM", "Negotiation"]
  }
]);

// Create products collection for multi-source testing
db.products.insertMany([
  {
    _id: ObjectId(),
    name: "Laptop Pro",
    description: "High-performance laptop",
    price: 1299.99,
    category: "Electronics",
    brand: "TechCorp",
    in_stock: true,
    quantity: 50,
    created_at: new Date(),
    updated_at: new Date()
  },
  {
    _id: ObjectId(),
    name: "Wireless Mouse",
    description: "Ergonomic wireless mouse",
    price: 29.99,
    category: "Accessories",
    brand: "TechCorp",
    in_stock: true,
    quantity: 200,
    created_at: new Date(),
    updated_at: new Date()
  }
]);

// Create orders collection
db.orders.insertMany([
  {
    _id: ObjectId(),
    order_id: "ORD-001",
    customer_id: "CUST-001",
    total_amount: 1329.98,
    status: "completed",
    order_date: new Date(),
    items: [
      { product_id: "PROD-001", quantity: 1, price: 1299.99 },
      { product_id: "PROD-002", quantity: 1, price: 29.99 }
    ],
    created_at: new Date(),
    updated_at: new Date()
  }
]);

// Create indexes for better performance
db.users.createIndex({ "email": 1 }, { unique: true });
db.users.createIndex({ "department": 1 });
db.users.createIndex({ "created_at": 1 });

db.products.createIndex({ "category": 1 });
db.products.createIndex({ "price": 1 });
db.products.createIndex({ "name": "text", "description": "text" });

db.orders.createIndex({ "customer_id": 1 });
db.orders.createIndex({ "order_date": 1 });
db.orders.createIndex({ "status": 1 });

print('MongoDB source database initialization completed successfully!');
print('Created collections: users, products, orders');
print('Sample data inserted and indexes created.');