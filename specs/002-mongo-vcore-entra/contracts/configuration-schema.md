# Configuration Contract

## Enhanced MongoDB Configuration Schema

### Position Tracking Configuration
```yaml
# Schema for position tracking with Entra authentication
position_tracking:
  type: "mongodb"
  mongo:
    connection_uri: string                    # Required: MongoDB connection URI (without credentials for Entra)
    database: string                          # Required: Database name
    collection: string                        # Optional: Collection name (default: "positions")
    
    # Authentication method selection
    auth_method: string                       # Required: "connection_string" | "entra"
    
    # Entra authentication configuration (required when auth_method = "entra")
    tenant_id: string                         # Required for Entra: Azure tenant ID
    client_id: string                         # Optional for Entra: User-assigned managed identity client ID
    scopes: []string                          # Optional for Entra: OAuth scopes (default: ["https://cosmos.azure.com/.default"])
    auth_timeout: duration                    # Optional: Authentication timeout (default: 30s)
    
    # Connection settings (existing)
    connect_timeout: duration
    server_selection_timeout: duration
    max_pool_size: int
    enable_transactions: bool
```

### Stream Source Configuration
```yaml
# Schema for stream source with Entra authentication
streams:
  stream_name:
    type: "mongodb"
    source:
      uri: string                             # Required: MongoDB connection URI
      database: string                        # Required: Source database
      collection: string                      # Optional: Source collection
      
      # Authentication configuration
      auth_method: string                     # Required: "connection_string" | "entra"
      auth_config:                           # Required when auth_method = "entra"
        tenant_id: string                     # Required: Azure tenant ID
        client_id: string                     # Optional: User-assigned managed identity
        scopes: []string                      # Optional: OAuth scopes
        timeout: duration                     # Optional: Auth timeout
        retry:                               # Optional: Retry configuration
          max_attempts: int
          initial_delay: duration
          max_delay: duration
          multiplier: float
```

### Destination Configuration
```yaml
# Schema for destination endpoint with Entra authentication
streams:
  stream_name:
    destinations:
      - type: "mongodb"
        config:
          mongo_uri: string                   # Required: MongoDB connection URI
          mongo_database_name: string         # Required: Target database
          mongo_collection_name: string       # Required: Target collection
          
          # Authentication configuration
          mongo_auth_method: string           # Required: "connection_string" | "entra"
          mongo_tenant_id: string             # Required for Entra: Azure tenant ID
          mongo_client_id: string             # Optional for Entra: Managed identity client ID
          mongo_scopes: []string              # Optional for Entra: OAuth scopes
```

## Validation Rules

### Authentication Method Validation
- `auth_method` MUST be either "connection_string" or "entra"
- When `auth_method` is "connection_string": URI may contain credentials
- When `auth_method` is "entra": URI MUST NOT contain credentials
- When `auth_method` is "entra": `tenant_id` is REQUIRED

### Entra Configuration Validation
- `tenant_id` MUST be valid UUID format when provided
- `client_id` MUST be valid UUID format when provided
- `scopes` MUST be array of valid OAuth scope URIs
- `scopes` MUST contain at least one scope when provided
- Default scope: `["https://cosmos.azure.com/.default"]`

### Connection URI Validation
- URI MUST be valid MongoDB connection string format
- For Entra auth: URI MUST NOT contain username/password components
- For connection string auth: URI MUST follow existing validation rules
- URI MUST specify valid hostname and port for Cosmos DB

### Timeout Configuration
- `auth_timeout` MUST be positive duration
- `timeout` MUST be positive duration
- Default timeout values MUST be reasonable (30s for auth, 10s for connections)

## Configuration Migration

### Backward Compatibility
- Existing configurations without `auth_method` default to "connection_string"
- Existing `connection_uri` fields remain valid for connection string auth
- No breaking changes to existing configuration files
- Gradual migration path available for existing deployments

### Migration Examples

#### Before (Connection String Only)
```yaml
position_tracking:
  type: "mongodb"
  mongo:
    connection_uri: "mongodb://user:pass@cosmos.mongo.cosmos.azure.com:10255/db"
    database: "positions"
```

#### After (Explicit Connection String)
```yaml
position_tracking:
  type: "mongodb"
  mongo:
    connection_uri: "mongodb://user:pass@cosmos.mongo.cosmos.azure.com:10255/db"
    database: "positions"
    auth_method: "connection_string"  # Explicit but optional
```

#### After (Entra Authentication)
```yaml
position_tracking:
  type: "mongodb"
  mongo:
    connection_uri: "mongodb://cosmos.mongo.cosmos.azure.com:10255/db"
    database: "positions"
    auth_method: "entra"
    tenant_id: "12345678-1234-1234-1234-123456789012"
    client_id: "87654321-4321-4321-4321-210987654321"
```