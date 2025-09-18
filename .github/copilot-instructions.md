# replicator Development Guidelines

Auto-generated from all feature plans. Last updated: 2025-09-17

## Active Technologies
- Go 1.25+ + MongoDB, Azure Entra ID (001-replicator-is-a)
- go.mongodb.org/mongo-driver + github.com/Azure/azure-sdk-for-go/sdk/azidentity (002-mongo-vcore-entra)
- MONGODB-OIDC mechanism + Azure workload identity (002-mongo-vcore-entra)

## Project Structure
```
pkg/
├── auth/           # Azure Entra authentication providers + shared MongoDB client
├── position/       # MongoDB position tracking with Entra auth
├── streams/        # MongoDB streams with enhanced authentication
├── estuary/        # MongoDB endpoints with Entra support
└── config/         # Configuration management
```

## Authentication Configuration
```yaml
# Azure Entra authentication for MongoDB Cosmos DB
source:
  type: "mongodb"
  uri: "mongodb://cosmos-cluster.mongo.cosmos.azure.com:10255/"
  options:
    auth_method: "entra"
    tenant_id: "12345678-1234-1234-1234-123456789012"
    client_id: "87654321-4321-4321-4321-210987654321"
    scopes: ["https://cosmos.azure.com/.default"]
    refresh_before_expiry: "5m"
```

## Commands
```bash
# Test Azure Entra authentication
./replicator --config config/stream-entra.yaml --test-auth

# Validate Entra configuration
./replicator --config config/stream-entra.yaml --validate-config

# Run with Entra authentication
./replicator --config config/stream-entra.yaml
```

## Code Style
- Follow standard golang conventions
- Use MONGO-OIDC authentication mechanism for Azure Cosmos DB
- Implement library-first architecture with CLI interfaces
- Use shared `NewMongoClientWithAuth()` for all MongoDB connections
- Ensure backward compatibility with connection string authentication
- Apply scope validation to prevent common security mistakes

## Recent Changes
- 002-mongo-vcore-entra: ✅ COMPLETED - Azure Entra authentication + MongoDB OIDC support with shared client
- 001-replicator-is-a: Added initial MongoDB + replication system

<!-- MANUAL ADDITIONS START -->
## Implementation Notes
- All MongoDB connections use `pkg/auth.NewMongoClientWithAuth()` for consistency
- Token caching and singleflight concurrency prevent auth stampede
- Scope validation enforces `https://cosmos.azure.com/.default` for Cosmos DB
- Backwards compatible: existing connection string auth unchanged
- Ready for Azure deployment with workload identity
<!-- MANUAL ADDITIONS END -->