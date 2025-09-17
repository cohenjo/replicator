# MongoDB Entra Authentication Contract

## Package: pkg/auth

### Interface: MongoEntraTokenProvider
```go
// MongoEntraTokenProvider provides Azure Entra tokens for MongoDB OIDC authentication
type MongoEntraTokenProvider interface {
    // GetMongoToken obtains a token specifically for MongoDB authentication
    GetMongoToken(ctx context.Context, resourceURI string) (*Credentials, error)
    
    // GetOIDCCredential returns MongoDB OIDC credential with Azure Entra token
    GetOIDCCredential(ctx context.Context, args *options.OIDCArgs) (*options.OIDCCredential, error)
    
    // ValidateMongoAccess validates token has sufficient permissions for MongoDB access
    ValidateMongoAccess(ctx context.Context, token string, resourceURI string) error
}
```

### Function Signature: NewMongoEntraProvider
```go
// NewMongoEntraProvider creates an Entra token provider configured for MongoDB authentication
func NewMongoEntraProvider(config MongoEntraConfig) (MongoEntraTokenProvider, error)
```

**Input Schema:**
```go
type MongoEntraConfig struct {
    TenantID     string        `json:"tenant_id"`
    ClientID     string        `json:"client_id,omitempty"`
    ResourceURI  string        `json:"resource_uri"`
    Scopes       []string      `json:"scopes"`
    Timeout      time.Duration `json:"timeout"`
    RetryConfig  RetryConfig   `json:"retry_config"`
}
```

**Output Schema:**
```go
type MongoEntraTokenProvider interface {
    GetMongoToken(ctx context.Context, resourceURI string) (*Credentials, error)
    GetOIDCCredential(ctx context.Context, args *options.OIDCArgs) (*options.OIDCCredential, error)
}
```

**Error Cases:**
- Invalid tenant ID format
- Missing required configuration
- Azure authentication failure
- Insufficient permissions for MongoDB access

## Package: pkg/position

### Function Signature: NewMongoTracker (Enhanced)
```go
// NewMongoTracker creates MongoDB position tracker with optional Entra authentication
func NewMongoTracker(config *MongoConfig) (*MongoTracker, error)
```

**Enhanced Input Schema:**
```go
type MongoConfig struct {
    // Existing fields
    ConnectionURI string `json:"connection_uri"`
    Database      string `json:"database"`
    Collection    string `json:"collection"`
    
    // New authentication fields
    AuthMethod    string           `json:"auth_method"`    // "connection_string" or "entra"
    TenantID      string           `json:"tenant_id,omitempty"`
    ClientID      string           `json:"client_id,omitempty"`
    Scopes        []string         `json:"scopes,omitempty"`
    AuthTimeout   time.Duration    `json:"auth_timeout,omitempty"`
}
```

**Contract Requirements:**
- MUST support both "connection_string" and "entra" auth methods
- MUST validate authentication configuration before connection
- MUST handle token refresh automatically during long operations
- MUST provide meaningful error messages for authentication failures

## Package: pkg/streams

### Function Signature: NewMongoDBStream (Enhanced)
```go
// NewMongoDBStream creates MongoDB change stream with optional Entra authentication
func NewMongoDBStream(config StreamConfig) (*MongoDBStream, error)
```

**Enhanced Stream Configuration:**
```go
type MongoStreamConfig struct {
    // Existing fields
    URI        string `json:"uri"`
    Database   string `json:"database"`
    Collection string `json:"collection"`
    
    // New authentication fields
    AuthMethod string           `json:"auth_method"`
    AuthConfig *EntraAuthConfig `json:"auth_config,omitempty"`
}

type EntraAuthConfig struct {
    TenantID    string        `json:"tenant_id"`
    ClientID    string        `json:"client_id,omitempty"`
    Scopes      []string      `json:"scopes"`
    Timeout     time.Duration `json:"timeout"`
}
```

**Contract Requirements:**
- MUST establish authenticated connection before creating change stream
- MUST handle authentication token renewal during stream operation
- MUST reconnect automatically on authentication failure
- MUST emit authentication events for observability

## Package: pkg/estuary

### Function Signature: NewMongoEndpoint (Enhanced)
```go
// NewMongoEndpoint creates MongoDB destination endpoint with optional Entra authentication
func NewMongoEndpoint(streamConfig *config.WaterFlowsConfig) (MongoEndpoint, error)
```

**Enhanced Configuration:**
```go
type WaterFlowsConfig struct {
    // Existing MongoDB fields
    MongoURI            string `json:"mongo_uri"`
    MongoDatabaseName   string `json:"mongo_database_name"`
    MongoCollectionName string `json:"mongo_collection_name"`
    
    // New authentication fields
    MongoAuthMethod     string           `json:"mongo_auth_method,omitempty"`
    MongoTenantID       string           `json:"mongo_tenant_id,omitempty"`
    MongoClientID       string           `json:"mongo_client_id,omitempty"`
    MongoScopes         []string         `json:"mongo_scopes,omitempty"`
}
```

**Contract Requirements:**
- MUST support backward compatibility with existing URI-based authentication
- MUST validate Entra configuration when auth method is "entra"
- MUST handle write operations with authenticated client
- MUST retry operations on authentication token expiry