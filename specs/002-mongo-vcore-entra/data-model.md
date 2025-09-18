# Data Model: Azure Entra Authentication for MongoDB Cosmos DB

## Enhanced Entities

### MongoConfig (Extended)
**Purpose**: Enhanced MongoDB configuration with authentication method selection
**Fields**:
- `connection_uri`: string - MongoDB connection URI (existing)
- `database`: string - Database name (existing)
- `collection`: string - Collection name (existing)
- `auth_method`: string - Authentication method: "connection_string" (default) or "entra"
- `tenant_id`: string - Azure tenant ID (for Entra auth, required when auth_method="entra")
- `client_id`: string - Azure client ID for user-assigned managed identity (optional)
- `scopes`: []string - OAuth scopes for authentication (default: ["https://cosmos.azure.com/.default"])
- `refresh_before_expiry`: time.Duration - Token refresh buffer (default: 5m)
- `connection_timeout`: time.Duration - Connection timeout (existing)
- `server_selection_timeout`: time.Duration - Server selection timeout (existing)

**Validation Rules**:
- auth_method must be "connection_string" or "entra"
- When auth_method is "entra": tenant_id is required
- When auth_method is "connection_string": existing validation rules apply
- connection_uri must not contain credentials when auth_method is "entra"
- scopes must be valid OAuth scope URIs when provided
- scopes must not contain incorrect service scopes (e.g., PostgreSQL/MySQL scopes)
- refresh_before_expiry must be positive duration when provided

**State Transitions**:
- Configuration parsing → Authentication method determination → Credential setup → Connection establishment

### OIDCCredentialProvider (Enhanced)
**Purpose**: Implements MongoDB MONGODB-OIDC callback for Azure Entra token provision
**Fields**:
- `auth_provider`: *AzureEntraProvider - Azure Entra token provider (reuse existing)
- `scopes`: []string - OAuth scopes for token requests
- `context`: context.Context - Request context for authentication
- `logger`: Logger - Structured logger for authentication events
- `correlation_id`: string - Request correlation ID for log tracing

**Validation Rules**:
- auth_provider must implement existing TokenProvider interface
- scopes must be non-empty array of valid OAuth scopes
- scopes must be Azure Cosmos DB scopes, not other Azure services
- context must be non-nil

**Relationships**:
- Uses existing auth.AzureEntraProvider from pkg/auth
- Implements MongoDB driver's MONGODB-OIDC callback interface
- Integrates with mongo.options.Credential

## Configuration Schema Extensions

### Stream Configuration
```yaml
streams:
  cosmos_source:
    type: "mongodb"
    source:
      # Connection details
      uri: "mongodb://cosmos-cluster.mongo.cosmos.azure.com:10255/"
      database: "production"
      collection: "events"
      
      # Authentication configuration
      auth_method: "entra"
      auth_config:
        tenant_id: "12345678-1234-1234-1234-123456789012"
        client_id: "87654321-4321-4321-4321-210987654321"  # optional
        scopes: ["https://cosmos.azure.com/.default"]
        timeout: "30s"
        retry:
          max_attempts: 3
          initial_delay: "1s"
          max_delay: "10s"
          multiplier: 2.0
```

### Position Tracking Configuration
```yaml
position_tracking:
  type: "mongodb"
  mongo:
    connection_uri: "mongodb://cosmos-positions.mongo.cosmos.azure.com:10255/"
    database: "replicator_positions"
    collection: "stream_positions"
    
    # Enhanced authentication
    auth_method: "entra"
    tenant_id: "12345678-1234-1234-1234-123456789012"
    client_id: "87654321-4321-4321-4321-210987654321"
    scopes: ["https://cosmos.azure.com/.default"]
```

## Error Types and Handling

### Authentication Errors
- `ErrInvalidAuthMethod`: Unsupported authentication method specified
- `ErrMissingTenantID`: Tenant ID required for Entra authentication
- `ErrInvalidScope`: Invalid OAuth scope format
- `ErrTokenAcquisitionFailed`: Failed to acquire Azure Entra token
- `ErrTokenExpired`: Authentication token has expired
- `ErrInsufficientPermissions`: Token lacks required permissions for MongoDB access

### Configuration Errors
- `ErrConflictingAuthConfig`: Connection string contains credentials with Entra auth method
- `ErrInvalidConnectionURI`: Malformed MongoDB connection URI
- `ErrMissingAuthConfig`: Auth configuration required when auth_method is "entra"

### Connection Errors
- `ErrAuthenticationFailed`: MongoDB authentication failed with provided credentials
- `ErrConnectionTimeout`: Connection timeout during authentication
- `ErrOIDCCallbackFailed`: OIDC callback function failed during authentication

## Metrics and Observability

### Authentication Metrics
- `replicator_mongo_auth_attempts_total`: Counter of authentication attempts by method
- `replicator_mongo_auth_success_total`: Counter of successful authentications by method
- `replicator_mongo_auth_failures_total`: Counter of authentication failures by error type
- `replicator_mongo_auth_duration_seconds`: Histogram of authentication duration
- `replicator_mongo_token_refresh_total`: Counter of token refresh operations
- `replicator_mongo_token_cache_hits_total`: Counter of token cache hits
- `replicator_mongo_token_cache_misses_total`: Counter of token cache misses
- `replicator_mongo_token_refresh_failures_total`: Counter of token refresh failures
- `replicator_mongo_auth_retry_attempts_total`: Counter of authentication retry attempts
- `replicator_mongo_connection_active_total`: Gauge of active MongoDB connections by auth method

### Log Events
- Authentication attempts with method, outcome, and correlation ID
- Token acquisition and refresh events with correlation ID
- Connection establishment and failure events
- Configuration validation errors
- Security audit events for credential access
- Cache hit/miss events for performance monitoring