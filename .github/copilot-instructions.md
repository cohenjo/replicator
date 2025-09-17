# replicator Development Guidelines

Auto-generated from all feature plans. Last updated: 2025-09-16

## Active Technologies
- Go 1.25+ + MongoDB, Azure Entra ID (001-replicator-is-a)
- go.mongodb.org/mongo-driver + github.com/Azure/azure-sdk-for-go/sdk/azidentity (002-mongo-vcore-entra)

## Project Structure
```
pkg/
├── auth/           # Azure Entra authentication providers
├── position/       # MongoDB position tracking with Entra auth
├── streams/        # MongoDB streams with enhanced authentication
├── estuary/        # MongoDB endpoints with Entra support
└── config/         # Configuration management
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
- Ensure backward compatibility with connection string authentication

## Recent Changes
- 002-mongo-vcore-entra: Added Azure Entra authentication + MongoDB OIDC support
- 001-replicator-is-a: Added initial MongoDB + replication system

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->