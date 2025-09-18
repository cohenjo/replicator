# Migration Guide: Connection String to Azure Entra Authentication

This guide helps you migrate from connection string authentication to Azure Entra authentication for MongoDB Cosmos DB.

## Overview

Azure Entra authentication provides enhanced security by eliminating the need for connection strings with embedded credentials. Instead, it uses Azure workload identity and OAuth tokens.

## Benefits of Migration

- ✅ **No Secrets**: Credentials managed by Azure, not stored in configuration
- ✅ **Token Rotation**: Automatic token refresh and expiry handling  
- ✅ **Audit Trail**: Enhanced logging and monitoring through Azure
- ✅ **Principle of Least Privilege**: Fine-grained permissions via Azure RBAC
- ✅ **Compliance**: Meets enterprise security requirements

## Prerequisites

1. **Azure Kubernetes Service (AKS)** with workload identity enabled
2. **Azure Entra application registration** with appropriate permissions
3. **MongoDB Cosmos DB vCore** cluster with AAD authentication enabled
4. **Role assignments** for the application (Cosmos DB Data Contributor)

## Step-by-Step Migration

### 1. Current Configuration (Connection String)

```yaml
# BEFORE: Connection string authentication
streams:
  - name: "mongo-stream"
    source:
      type: "mongodb"
      uri: "mongodb://username:password@cosmos-cluster.mongo.cosmos.azure.com:10255/"
      database: "production"
```

### 2. Azure Setup

#### Create Application Registration
```bash
# Create Azure Entra application
az ad app create \
  --display-name "replicator-cosmos-auth" \
  --web-home-page-url "https://replicator.example.com"

# Note the client_id from output
export CLIENT_ID="87654321-4321-4321-4321-210987654321"
export TENANT_ID="12345678-1234-1234-1234-123456789012"
```

#### Configure Workload Identity
```bash
# Create federated credential for Kubernetes service account
az ad app federated-credential create \
  --id $CLIENT_ID \
  --parameters '{
    "name": "replicator-k8s-credential",
    "issuer": "https://oidc.prod-aks.azure.com/OIDC_ISSUER_ID/",
    "subject": "system:serviceaccount:replicator:replicator-service-account",
    "description": "Kubernetes service account federated credential",
    "audiences": ["api://AzureADTokenExchange"]
  }'
```

#### Assign Cosmos DB Permissions
```bash
# Get Cosmos DB resource ID
export COSMOS_RESOURCE_ID="/subscriptions/SUBSCRIPTION_ID/resourceGroups/RG_NAME/providers/Microsoft.DocumentDB/databaseAccounts/COSMOS_ACCOUNT"

# Assign Cosmos DB Data Contributor role
az role assignment create \
  --assignee $CLIENT_ID \
  --role "b7e6dc6d-f1e8-4753-8033-0f276bb0955b" \
  --scope $COSMOS_RESOURCE_ID
```

### 3. Update Configuration

#### New Configuration (Entra Authentication)
```yaml
# AFTER: Azure Entra authentication
streams:
  - name: "mongo-stream"
    source:
      type: "mongodb"
      # Remove credentials from URI
      uri: "mongodb://cosmos-cluster.mongo.cosmos.azure.com:10255/"
      database: "production"
      options:
        # Add Entra authentication
        auth_method: "entra"
        tenant_id: "12345678-1234-1234-1234-123456789012"
        client_id: "87654321-4321-4321-4321-210987654321"
        scopes:
          - "https://cosmos.azure.com/.default"
        refresh_before_expiry: "5m"
```

### 4. Kubernetes Configuration

#### Service Account with Workload Identity
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: replicator-service-account
  namespace: replicator
  annotations:
    azure.workload.identity/client-id: "87654321-4321-4321-4321-210987654321"
    azure.workload.identity/tenant-id: "12345678-1234-1234-1234-123456789012"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: replicator
spec:
  template:
    metadata:
      labels:
        azure.workload.identity/use: "true"
    spec:
      serviceAccountName: replicator-service-account
      containers:
      - name: replicator
        image: replicator:latest
        env:
        - name: AZURE_CLIENT_ID
          value: "87654321-4321-4321-4321-210987654321"
        - name: AZURE_TENANT_ID  
          value: "12345678-1234-1234-1234-123456789012"
```

## Migration Strategies

### Strategy 1: Blue-Green Deployment
1. Deploy new version with Entra auth alongside existing version
2. Gradually shift traffic to new version
3. Monitor for authentication issues
4. Decommission old version once stable

### Strategy 2: Rolling Update
1. Update configuration in staging environment first
2. Test authentication flow thoroughly
3. Apply configuration update to production
4. Monitor authentication metrics

### Strategy 3: Canary Deployment
1. Configure subset of streams with Entra auth
2. Monitor authentication success rates
3. Gradually migrate remaining streams
4. Complete migration once confidence is established

## Validation and Testing

### Test Authentication Configuration
```bash
# Validate configuration before deployment
./replicator --config config/cosmos-entra.yaml --validate-config

# Test authentication without starting streams
./replicator --config config/cosmos-entra.yaml --test-auth
```

### Monitor Authentication Health
```bash
# Check authentication metrics
curl http://replicator:9090/metrics | grep mongo_auth

# Example metrics to monitor
replicator_mongo_auth_success_total
replicator_mongo_auth_failure_total  
replicator_mongo_token_refresh_total
replicator_mongo_token_cache_hits_total
```

### Verify Token Lifecycle
```yaml
# Enable debug logging for authentication
logging:
  level: "debug"
  
# Monitor logs for token events
kubectl logs -f deployment/replicator | grep -E "(token|auth|oidc)"
```

## Troubleshooting

### Common Issues

#### 1. Authentication Failure
```
Error: failed to initialize auth manager: failed to create workload identity credential: no token file specified
```

**Solution**: Ensure workload identity is properly configured and service account has correct annotations.

#### 2. Invalid Scope Error
```
Error: invalid scope for Azure Cosmos DB: https://ossrdbms-aad.database.windows.net/.default
```

**Solution**: Use correct Cosmos DB scope: `https://cosmos.azure.com/.default`

#### 3. Token Expiry Issues
```
Error: authentication failed: token expired
```

**Solution**: Check `refresh_before_expiry` setting and ensure token refresh is working.

### Debug Commands

```bash
# Check workload identity configuration
az aks show --resource-group $RG --name $AKS_NAME --query "oidcIssuerProfile"

# Verify federated credentials
az ad app federated-credential list --id $CLIENT_ID

# Test token acquisition manually
az account get-access-token --resource https://cosmos.azure.com/.default
```

## Performance Considerations

### Token Caching
- Tokens are cached in memory for optimal performance
- Cache hit ratio should be >95% for healthy operation
- Monitor `replicator_mongo_token_cache_hits_total` metric

### Connection Overhead
- Initial connection: +50-100ms (token acquisition)
- Subsequent connections: Same as connection string (cached tokens)
- Token refresh: Background operation, non-blocking

### Recommended Settings
```yaml
source:
  options:
    refresh_before_expiry: "5m"    # Refresh 5 minutes before expiry
    # Avoid too frequent refresh (< 1m) to prevent unnecessary load
```

## Rollback Plan

If issues occur during migration, you can quickly rollback:

### Quick Rollback
```yaml
# Revert to connection string authentication
streams:
  - name: "mongo-stream"
    source:
      type: "mongodb"
      uri: "mongodb://username:password@cosmos-cluster.mongo.cosmos.azure.com:10255/"
      database: "production"
      # Remove or comment out options section
      # options:
      #   auth_method: "entra"
```

### Gradual Rollback
1. Update configuration to use connection string auth
2. Redeploy application
3. Monitor for restored functionality
4. Investigate Entra auth issues offline

## Security Best Practices

1. **Least Privilege**: Only assign necessary Cosmos DB permissions
2. **Environment Separation**: Use different app registrations for dev/staging/prod
3. **Monitoring**: Enable authentication metrics and alerting
4. **Rotation**: Regularly rotate federated credentials
5. **Audit**: Monitor Azure AD sign-in logs for suspicious activity

## Conclusion

Migrating to Azure Entra authentication enhances security while maintaining operational simplicity. The process is designed to be backward-compatible and supports gradual migration strategies.

For additional support, see:
- [MongoDB Entra Implementation Guide](../docs/MONGODB_ENTRA_IMPLEMENTATION.md)
- [Azure Workload Identity Documentation](https://docs.microsoft.com/en-us/azure/aks/workload-identity-overview)
- [Cosmos DB AAD Authentication](https://docs.microsoft.com/en-us/azure/cosmos-db/how-to-setup-managed-identity)