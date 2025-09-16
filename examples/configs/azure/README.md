# Azure Configuration README
# This directory contains Azure-specific configuration templates

## ⚠️ IMPORTANT SECURITY NOTICE

**DO NOT commit actual Azure configuration files with credentials to version control!**

This directory contains templates that should be:
1. Copied outside the repository
2. Filled with actual Azure resource details
3. Used with proper environment variables
4. Secured with appropriate access controls

## Configuration Templates

### cosmos-to-eventhub.yaml.template
**Use Case**: Stream changes from CosmosDB to Azure Event Hubs
- **Source**: Azure CosmosDB with Change Feed
- **Target**: Azure Event Hubs
- **Authentication**: Azure Entra ID (Service Principal)
- **Features**: Real-time event streaming, partitioning, dead letter queues

### sql-to-cosmos.yaml.template  
**Use Case**: Replicate Azure SQL Database to CosmosDB for analytics
- **Source**: Azure SQL Database with Change Tracking
- **Target**: Azure CosmosDB
- **Authentication**: Azure Entra ID (Service Principal)
- **Features**: Data enrichment, customer segmentation, upsert operations

## Setup Instructions

### 1. Create Azure Service Principal

```bash
# Create service principal
az ad sp create-for-rbac --name "replicator-sp" --role contributor --scopes /subscriptions/{subscription-id}

# Note down:
# - appId (AZURE_CLIENT_ID)
# - password (AZURE_CLIENT_SECRET)  
# - tenant (AZURE_TENANT_ID)
```

### 2. Grant Required Permissions

**For CosmosDB:**
```bash
# Assign CosmosDB Data Contributor role
az cosmosdb sql role assignment create \
  --account-name YOUR_COSMOS_ACCOUNT \
  --resource-group YOUR_RG \
  --scope "/" \
  --principal-id {service-principal-object-id} \
  --role-definition-id "b24988ac-6180-42a0-ab88-20f7382dd24c"
```

**For Event Hubs:**
```bash
# Assign Event Hubs Data Owner role
az role assignment create \
  --assignee {service-principal-client-id} \
  --role "Azure Event Hubs Data Owner" \
  --scope /subscriptions/{subscription-id}/resourceGroups/{rg}/providers/Microsoft.EventHub/namespaces/{namespace}
```

**For Azure SQL:**
```bash
# Create contained database user and grant permissions
sqlcmd -S YOUR_SERVER.database.windows.net -d YOUR_DATABASE -G -l 30 -Q "
CREATE USER [replicator-sp] FROM EXTERNAL PROVIDER;
ALTER ROLE db_datareader ADD MEMBER [replicator-sp];
GRANT VIEW CHANGE TRACKING ON SCHEMA::dbo TO [replicator-sp];
"
```

### 3. Configure Environment Variables

Create a secure `.env` file (NOT in git):

```bash
# Azure Authentication
AZURE_TENANT_ID=your-tenant-id
AZURE_CLIENT_ID=your-client-id
AZURE_CLIENT_SECRET=your-client-secret

# CosmosDB
COSMOS_PRIMARY_KEY=your-cosmos-primary-key

# Event Hubs
EVENTHUB_KEY=your-eventhub-key

# Monitoring
AZURE_LOG_ANALYTICS_WORKSPACE_ID=your-workspace-id
APPLICATION_INSIGHTS_KEY=your-app-insights-key
```

### 4. Copy and Customize Templates

```bash
# Copy template outside repository
cp examples/configs/azure/cosmos-to-eventhub.yaml.template \
   /secure/location/cosmos-to-eventhub.yaml

# Edit configuration file
nano /secure/location/cosmos-to-eventhub.yaml

# Replace all YOUR_* placeholders with actual Azure resource names
```

### 5. Deploy Securely

```bash
# Use Azure Key Vault for secrets
az keyvault secret set --vault-name YOUR_VAULT --name "cosmos-key" --value "actual-key"

# Or use Kubernetes secrets
kubectl create secret generic azure-secrets \
  --from-literal=tenant-id="${AZURE_TENANT_ID}" \
  --from-literal=client-id="${AZURE_CLIENT_ID}" \
  --from-literal=client-secret="${AZURE_CLIENT_SECRET}"

# Deploy with Helm using external config
helm install replicator-azure ./deploy/helm/replicator \
  --values /secure/location/azure-values.yaml \
  --set-file config.configFile=/secure/location/cosmos-to-eventhub.yaml
```

## Security Best Practices

### 1. Use Managed Identity When Possible
- Prefer Azure Managed Identity over Service Principal
- Use System-assigned identity for Azure resources
- Avoid storing credentials in configuration files

### 2. Implement Least Privilege Access
- Grant minimal required permissions
- Use resource-specific roles instead of broad permissions
- Regularly audit and rotate credentials

### 3. Monitor and Alert
- Enable Azure Monitor integration
- Set up alerts for authentication failures
- Monitor unusual access patterns

### 4. Network Security
- Use Private Endpoints for Azure services
- Implement VNet integration for enhanced security
- Consider Azure Private Link for data transfers

## Troubleshooting Azure Authentication

### Common Issues

**Authentication failures:**
```bash
# Verify service principal exists
az ad sp show --id YOUR_CLIENT_ID

# Check role assignments
az role assignment list --assignee YOUR_CLIENT_ID

# Test authentication
az login --service-principal -u YOUR_CLIENT_ID -p YOUR_CLIENT_SECRET --tenant YOUR_TENANT_ID
```

**Permission errors:**
```bash
# Check CosmosDB permissions
az cosmosdb sql role assignment list --account-name YOUR_ACCOUNT --resource-group YOUR_RG

# Verify Event Hubs access
az eventhubs namespace authorization-rule list --resource-group YOUR_RG --namespace-name YOUR_NAMESPACE
```

**Configuration validation:**
```bash
# Validate YAML syntax
python -c "import yaml; yaml.safe_load(open('your-config.yaml'))"

# Test connectivity
./replicator --config your-config.yaml --validate-only
```

## Production Considerations

### High Availability
- Deploy across multiple Azure regions
- Use Azure Traffic Manager for load balancing
- Implement disaster recovery procedures

### Performance Optimization
- Configure appropriate batch sizes for your workload
- Use CosmosDB autoscale for variable workloads
- Monitor and tune Event Hubs throughput units

### Cost Management
- Use CosmosDB serverless for development/testing
- Configure Event Hubs auto-inflate
- Monitor costs with Azure Cost Management

### Compliance
- Ensure data residency requirements are met
- Implement audit logging for compliance
- Use Azure Policy for governance

For additional help, contact your Azure administrator or refer to Azure documentation.