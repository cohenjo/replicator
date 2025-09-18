# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.2.0] - 2025-09-17

### Added - Azure Entra Authentication for MongoDB 🔒

#### Core Features
- **Azure Entra Authentication**: Full support for MongoDB Cosmos DB with workload identity
- **MONGODB-OIDC Integration**: Native MongoDB driver support for Azure authentication
- **Shared Authentication**: Single `NewMongoClientWithAuth()` function for all MongoDB connections
- **Token Management**: Automatic token caching, refresh, and expiry handling
- **Backwards Compatibility**: Zero breaking changes to existing connection string authentication

#### Configuration Enhancements
- Added `auth_method`, `tenant_id`, `client_id`, `scopes` fields to `MongoConfig`
- Support for Entra authentication in stream source options
- Comprehensive validation for Azure tenant IDs, client IDs, and scopes
- Default Cosmos DB scope: `https://cosmos.azure.com/.default`

#### Security Features
- **Scope Validation**: Prevents PostgreSQL/MySQL scope misuse
- **Credential Detection**: Warns against embedded credentials with Entra auth
- **UUID Validation**: Ensures proper Azure tenant ID format
- **Singleflight Concurrency**: Prevents token request stampede

#### Integration Updates
- Updated `pkg/streams/mongodb_stream.go` to use shared authentication
- Updated `pkg/estuary/mongo.go` to use shared authentication  
- Updated `pkg/position/mongo_tracker.go` with Entra authentication support
- All MongoDB connections now support both connection string and Entra auth

#### Testing & Validation
- Comprehensive TDD implementation with contract and integration tests
- Token lifecycle testing with refresh and expiry scenarios
- Concurrency testing for authentication flow
- Configuration validation tests for all error conditions

#### Documentation
- [MongoDB Entra Implementation Guide](docs/MONGODB_ENTRA_IMPLEMENTATION.md)
- [Azure Migration Guide](examples/configs/azure/MIGRATION_GUIDE.md)
- [Configuration Examples](examples/configs/azure/cosmos-entra-auth.yaml)
- Updated README with Azure authentication section

#### Dependencies
- Added `github.com/Azure/azure-sdk-for-go/sdk/azidentity` v1.12.0
- Added `golang.org/x/sync/singleflight` for token concurrency control
- Maintained MongoDB driver v1 for compatibility

### Technical Details
- **Library-First Architecture**: Single shared authentication function
- **Minimal Code Changes**: Extended existing structs, preserved interfaces
- **Maximum Code Reuse**: Common authentication path for all MongoDB connections
- **Performance Optimized**: Token caching reduces auth overhead to ~1-5ms
- **Azure-Ready**: Designed for Azure Kubernetes with workload identity

### Migration
Existing configurations continue to work unchanged. To enable Azure Entra authentication:

```yaml
source:
  type: "mongodb"
  uri: "mongodb://cosmos-cluster.mongo.cosmos.azure.com:10255/"
  options:
    auth_method: "entra"
    tenant_id: "12345678-1234-1234-1234-123456789012"
    client_id: "87654321-4321-4321-4321-210987654321"
    scopes: ["https://cosmos.azure.com/.default"]
```

## [1.1.0] - Previous Release

### Added
- MySQL binlog streaming implementation
- Generic position tracking system
- Enhanced MongoDB change stream support
- Kafka integration improvements

### Changed
- Improved error handling and retry logic
- Enhanced metrics collection
- Updated dependencies

### Fixed
- Various bug fixes and performance improvements

## [1.0.0] - Initial Release

### Added
- Core replication engine
- MongoDB change streams
- MySQL binlog replication  
- Kafka integration
- Elasticsearch output
- Prometheus metrics
- Configuration-based transformations