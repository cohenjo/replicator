# Research: Azure Entra Authentication for MongoDB Cosmos DB

## Research Findings

### MONGODB-OIDC Authentication Mechanism
**Decision**: Use MongoDB's OIDC authentication mechanism with Azure Entra tokens
**Rationale**: 
- MongoDB driver supports OIDC authentication via `MONGODB-OIDC` auth mechanism
- Azure Cosmos DB for MongoDB vCore supports Azure Entra authentication through OIDC
- Seamless integration with existing Azure workload identity infrastructure
- Standard OAuth 2.0/OIDC flow provides secure token-based authentication

**Alternatives considered**:
- Custom authentication wrapper: Rejected due to complexity and non-standard approach
- Connection string with embedded tokens: Rejected due to security concerns and token expiration issues
- Direct Azure SDK integration in mongo client: Rejected due to tight coupling

### MongoDB Driver OIDC Integration Pattern
**Decision**: Use `options.Credential` with `AuthMechanism: "MONGODB-OIDC"` and custom token callback
**Rationale**:
- Official MongoDB Go driver supports OIDC via credential options
- Callback-based token provision allows for automatic token refresh
- Clean separation between authentication logic and database connection
- Follows MongoDB best practices for external authentication providers

**Implementation Pattern**:
```go
credential := options.Credential{
    AuthMechanism: "MONGODB-OIDC",
    Username:      "", // Empty for MONGODB-OIDC
    OIDCMachineCallback: func(ctx context.Context, args *options.OIDCArgs) (*options.OIDCCredential, error) {
        // Use pkg/auth to get Azure Entra token
        token, err := authProvider.GetToken(ctx, scopes)
        return &options.OIDCCredential{
            AccessToken: token.AccessToken,
        }, err
    },
}
```

### Azure Cosmos DB Scope Requirements
**Decision**: Use Azure Cosmos DB specific OAuth scopes
**Rationale**:
- Azure Cosmos DB requires specific scopes for authentication
- Standard scope: `https://cosmos.azure.com/.default` (Azure Cosmos DB for MongoDB vCore)
- Resource-specific permissions managed through Azure RBAC
- Workload identity must have appropriate Cosmos DB role assignments
- **WARNING**: Do NOT use `https://ossrdbms-aad.database.windows.net/.default` (PostgreSQL/MySQL scope)

**Scope Configuration**:
- Default scope: `https://cosmos.azure.com/.default`
- Configurable scopes for custom resource access
- Validation of scope format during configuration
- Scope validation to prevent common mistakes (wrong service scopes)

### Configuration Strategy
**Decision**: Extend existing MongoDB configuration with optional authentication method selection
**Rationale**:
- Backward compatibility with existing connection string configurations
- Clean separation between connection details and authentication method
- Consistent with existing auth package patterns
- Allows mixed authentication methods in multi-source configurations

**Configuration Structure**:
```yaml
mongo:
  connection_uri: "mongodb://cosmos-cluster.mongo.cosmos.azure.com:10255/"
  auth_method: "entra"  # or "connection_string" (default)
  auth_config:
    tenant_id: "tenant-uuid"
    client_id: "client-uuid"  # for user-assigned managed identity
    scopes: ["https://cosmos.azure.com/.default"]
```

### Error Handling and Retry Strategy
**Decision**: Implement comprehensive error handling with categorized retry strategies
**Rationale**:
- Azure authentication can fail due to various reasons (token expiration, network issues, permission changes)
- MongoDB connection failures need different handling than authentication failures
- Observability requirements demand detailed error categorization

**Error Categories**:
- Authentication errors: Token acquisition failures, invalid credentials
- Authorization errors: Insufficient permissions, RBAC issues
- Network errors: Transient connectivity issues
- Configuration errors: Invalid tenant/client IDs, malformed URIs

### Token Lifecycle Management
**Decision**: Leverage existing pkg/auth token caching and refresh mechanisms
**Rationale**:
- pkg/auth already implements token caching and automatic refresh
- MongoDB OIDC callback mechanism handles token refresh transparently
- Prevents unnecessary token requests during long-running operations
- Consistent with other Azure service integrations

**Token Management**:
- Automatic token refresh before expiration (5-minute buffer)
- Cache tokens per connection configuration
- Handle token revocation gracefully
- Metric collection for token operations

### Testing Strategy
**Decision**: Multi-layered testing with real Azure authentication simulation
**Rationale**:
- Contract tests ensure proper OIDC integration
- Integration tests validate end-to-end authentication flow
- Unit tests verify configuration parsing and error handling
- Testcontainers for MongoDB with authentication simulation

**Test Layers**:
1. Contract tests: MongoDB OIDC callback interface
2. Integration tests: Azure authentication flow simulation
3. Unit tests: Configuration validation and error scenarios
4. End-to-end tests: Full replication flow with Entra authentication

### Dependencies and Compatibility
**Decision**: Minimal additional dependencies, reuse existing Azure SDK integration
**Rationale**:
- pkg/auth already imports Azure SDK for Go
- MongoDB driver already imported in multiple packages
- No breaking changes to existing interfaces
- Maintains current performance characteristics

**Dependency Analysis**:
- No new external dependencies required
- Enhanced configuration structures only
- Backward compatible API extensions
- Consistent error handling patterns

### Security Considerations
**Decision**: Follow Azure security best practices with comprehensive audit logging
**Rationale**:
- Workload identity provides enhanced security over connection strings
- Audit logs required for compliance and troubleshooting
- Secure credential handling prevents token leakage
- Integration with existing observability infrastructure

**Security Measures**:
- No credential storage in configuration files
- Audit logging of authentication events
- Secure token caching with expiration
- Error messages that don't leak sensitive information