# Quickstart: Azure Entra Authentication for MongoDB Cosmos DB

## Prerequisites

### Azure Resources
- Azure Cosmos DB for MongoDB vCore cluster with Entra authentication enabled
- Azure Entra tenant with appropriate permissions
- User-assigned managed identity (optional) or system-assigned managed identity configured
- Proper RBAC roles assigned to managed identity for Cosmos DB access

### Required RBAC Roles
- `Cosmos DB Account Reader Role` - Read Cosmos DB account properties
- `Cosmos DB Operator` - Manage Cosmos DB resources
- `DocumentDB Account Contributor` - Full access to Cosmos DB account

### Environment Setup
- Kubernetes cluster with workload identity configured (for production)
- Azure CLI authenticated (for local development)
- Go 1.25+ development environment

## Quick Configuration Examples

### 1. Position Tracking with Entra Auth
```yaml
# config/position-tracking-entra.yaml
position_tracking:
  type: "mongodb"
  mongo:
    connection_uri: "mongodb://cosmos-cluster.mongo.cosmos.azure.com:10255/"
    database: "replicator_positions"
    collection: "stream_positions"
    auth_method: "entra"
    tenant_id: "12345678-1234-1234-1234-123456789012"
    # client_id: "87654321-4321-4321-4321-210987654321"  # Optional: for user-assigned identity
    scopes: ["https://cosmos.azure.com/.default"]  # CORRECT scope for Azure Cosmos DB
    refresh_before_expiry: "5m"  # Token refresh buffer
    enable_transactions: true
```

### 2. Stream Source with Entra Auth
```yaml
# config/stream-entra.yaml
streams:
  cosmos_events:
    type: "mongodb"
    source:
      uri: "mongodb://cosmos-source.mongo.cosmos.azure.com:10255/"
      database: "production"
      collection: "events"
      auth_method: "entra"
      tenant_id: "12345678-1234-1234-1234-123456789012"
      client_id: "87654321-4321-4321-4321-210987654321"
      scopes: ["https://cosmos.azure.com/.default"]  # MUST be Cosmos DB scope
      refresh_before_expiry: "5m"
```

### 3. Destination Endpoint with Entra Auth
```yaml
# config/destination-entra.yaml
streams:
  cosmos_replication:
    destinations:
      - type: "mongodb"
        config:
          mongo_uri: "mongodb://cosmos-target.mongo.cosmos.azure.com:10255/"
          mongo_database_name: "replicated_data"
          mongo_collection_name: "events"
          mongo_auth_method: "entra"
          mongo_tenant_id: "12345678-1234-1234-1234-123456789012"
          mongo_scopes: ["https://cosmos.azure.com/.default"]  # NOT PostgreSQL scope!
```

## Step-by-Step Setup

### Step 1: Configure Managed Identity
```bash
# Create user-assigned managed identity (if needed)
az identity create \
  --name replicator-identity \
  --resource-group replicator-rg \
  --location eastus

# Get the identity details
IDENTITY_ID=$(az identity show \
  --name replicator-identity \
  --resource-group replicator-rg \
  --query clientId -o tsv)

PRINCIPAL_ID=$(az identity show \
  --name replicator-identity \
  --resource-group replicator-rg \
  --query principalId -o tsv)
```

### Step 2: Assign Cosmos DB Permissions
```bash
# Get Cosmos DB resource ID
COSMOS_RESOURCE_ID=$(az cosmosdb show \
  --name cosmos-cluster \
  --resource-group cosmos-rg \
  --query id -o tsv)

# Assign DocumentDB Account Contributor role
az role assignment create \
  --assignee $PRINCIPAL_ID \
  --role "DocumentDB Account Contributor" \
  --scope $COSMOS_RESOURCE_ID
```

### Step 3: Configure Workload Identity (Kubernetes)
```yaml
# k8s/workload-identity.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: replicator-sa
  namespace: replicator
  annotations:
    azure.workload.identity/client-id: "87654321-4321-4321-4321-210987654321"
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
      serviceAccountName: replicator-sa
      containers:
      - name: replicator
        image: replicator:latest
        env:
        - name: AZURE_CLIENT_ID
          value: "87654321-4321-4321-4321-210987654321"
        - name: AZURE_TENANT_ID  
          value: "12345678-1234-1234-1234-123456789012"
```

### Step 4: Test Authentication
```go
// test/auth_test.go - Validation test
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/cohenjo/replicator/pkg/auth"
    "github.com/cohenjo/replicator/pkg/position"
)

func testEntraAuth() {
    config := &position.MongoConfig{
        ConnectionURI: "mongodb://cosmos-cluster.mongo.cosmos.azure.com:10255/",
        Database:      "test_db",
        Collection:    "test_collection",
        AuthMethod:    "entra",
        TenantID:      "12345678-1234-1234-1234-123456789012",
        ClientID:      "87654321-4321-4321-4321-210987654321",
        Scopes:        []string{"https://cosmos.azure.com/.default"},
    }
    
    tracker, err := position.NewMongoTracker(config)
    if err != nil {
        log.Fatalf("Failed to create tracker: %v", err)
    }
    defer tracker.Close()
    
    // Test basic operations
    position := map[string]interface{}{
        "stream_id": "test_stream",
        "position":  "12345",
        "timestamp": time.Now(),
    }
    
    err = tracker.Save(context.Background(), "test_stream", position)
    if err != nil {
        log.Fatalf("Failed to save position: %v", err)
    }
    
    fmt.Println("✅ Entra authentication working correctly!")
}
```

### Step 5: Run Integration Test
```bash
# Set environment variables for local testing
export AZURE_TENANT_ID="12345678-1234-1234-1234-123456789012"
export AZURE_CLIENT_ID="87654321-4321-4321-4321-210987654321"

# Run the replicator with Entra configuration
./replicator --config config/stream-entra.yaml

# Verify authentication in logs
grep "Successfully authenticated with Azure Entra" logs/replicator.log
```

## Troubleshooting

### Common Authentication Issues

#### 1. Token Acquisition Failed
```
Error: failed to get access token: ManagedIdentityCredential authentication failed
```
**Solution**: Verify managed identity is properly configured and assigned to the resource.

#### 2. Insufficient Permissions
```
Error: authentication failed: insufficient permissions for MongoDB access  
```
**Solution**: Ensure proper RBAC roles are assigned to the managed identity.

#### 3. Invalid Tenant ID
```
Error: invalid Azure Entra configuration: tenant ID must be valid UUID format
```
**Solution**: Verify tenant ID is correctly specified in configuration and is valid UUID.

#### 4. Wrong OAuth Scope
```
Error: authentication failed: invalid scope for Azure Cosmos DB
```
**Solution**: Use `https://cosmos.azure.com/.default` for Cosmos DB, NOT `https://ossrdbms-aad.database.windows.net/.default` (PostgreSQL/MySQL scope).

#### 5. Mongo Driver Version Issue
```
Error: MONGODB-OIDC authentication mechanism not supported
```
**Solution**: Upgrade to mongo-driver v2+ which supports MONGODB-OIDC authentication mechanism.

### Verification Commands

#### Check Managed Identity Assignment
```bash
# Verify identity exists
az identity show --name replicator-identity --resource-group replicator-rg

# Check role assignments
az role assignment list --assignee $PRINCIPAL_ID --scope $COSMOS_RESOURCE_ID
```

#### Validate Configuration
```bash
# Test configuration parsing
./replicator --config config/stream-entra.yaml --validate-config

# Check authentication connectivity
./replicator --config config/stream-entra.yaml --test-auth
```

#### Monitor Authentication Events
```bash
# View authentication logs
kubectl logs deployment/replicator | grep "auth"

# Check metrics for authentication success rate
curl http://localhost:8889/metrics | grep mongo_auth
```

## Migration from Connection Strings

### Gradual Migration Strategy
1. **Phase 1**: Add Entra configuration alongside existing connection string
2. **Phase 2**: Test Entra authentication in non-production environments  
3. **Phase 3**: Switch to Entra authentication in production
4. **Phase 4**: Remove connection string configuration

### Migration Example
```yaml
# Before migration
position_tracking:
  mongo:
    connection_uri: "mongodb://user:pass@cosmos.mongo.cosmos.azure.com:10255/"

# During migration (both methods available)
position_tracking:
  mongo:
    connection_uri: "mongodb://user:pass@cosmos.mongo.cosmos.azure.com:10255/"
    auth_method: "connection_string"  # Current
    # Prepare Entra config for future migration
    tenant_id: "12345678-1234-1234-1234-123456789012"
    client_id: "87654321-4321-4321-4321-210987654321"

# After migration
position_tracking:
  mongo:
    connection_uri: "mongodb://cosmos.mongo.cosmos.azure.com:10255/"
    auth_method: "entra"
    tenant_id: "12345678-1234-1234-1234-123456789012"
    client_id: "87654321-4321-4321-4321-210987654321"
```