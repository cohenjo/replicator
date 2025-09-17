package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/singleflight"
)

// MongoAuthConfig represents the configuration for MongoDB authentication
type MongoAuthConfig struct {
	// ConnectionURI for MongoDB connection
	ConnectionURI string
	
	// AuthMethod specifies the authentication method: "connection_string" or "entra"
	AuthMethod string
	
	// TenantID for Azure Entra authentication
	TenantID string
	
	// ClientID for Azure Entra authentication  
	ClientID string
	
	// Scopes for Azure Entra authentication
	Scopes []string
	
	// RefreshBeforeExpiry specifies how long before token expiry to refresh
	RefreshBeforeExpiry time.Duration
}

// tokenCacheEntry represents a cached token with metadata
type tokenCacheEntry struct {
	token     azcore.AccessToken
	expiresAt time.Time
	mutex     sync.RWMutex
}

// mongoAuthManager manages MongoDB authentication tokens
type mongoAuthManager struct {
	credential azcore.TokenCredential
	scopes     []string
	cache      map[string]*tokenCacheEntry
	cacheMutex sync.RWMutex
	group      singleflight.Group
	
	// refreshBeforeExpiry determines when to refresh tokens
	refreshBeforeExpiry time.Duration
}

var (
	// Global auth manager instance
	authManager *mongoAuthManager
	authOnce    sync.Once
)

// NewMongoClientWithAuth creates a new MongoDB client with the specified authentication method
func NewMongoClientWithAuth(ctx context.Context, config *MongoAuthConfig) (*mongo.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("mongo auth config is required")
	}
	
	if config.ConnectionURI == "" {
		return nil, fmt.Errorf("connection URI is required")
	}
	
	// Set default auth method to MONGODB-OIDC (Entra)
	if config.AuthMethod == "" {
		config.AuthMethod = "entra"
	}
	
	// Create client options with default parameters
	clientOpts := options.Client().ApplyURI(config.ConnectionURI)
	
	// Apply default connection parameters
	applyDefaultConnectionParams(clientOpts)
	
	switch config.AuthMethod {
	case "connection_string":
		// Use connection string authentication (existing behavior)
		return mongo.Connect(ctx, clientOpts)
		
	case "entra":
		// Validate Entra configuration
		if err := validateEntraConfig(config); err != nil {
			return nil, fmt.Errorf("invalid Entra configuration: %w", err)
		}
		
		// Set up Entra authentication
		return connectWithEntraAuth(ctx, clientOpts, config)
		
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", config.AuthMethod)
	}
}

// applyDefaultConnectionParams applies default MongoDB connection parameters
func applyDefaultConnectionParams(clientOpts *options.ClientOptions) {
	// Set connection timeouts
	connectTimeout := 30 * time.Second
	serverSelectionTimeout := 30 * time.Second
	
	clientOpts.SetConnectTimeout(connectTimeout)
	clientOpts.SetServerSelectionTimeout(serverSelectionTimeout)
	
	// Enable TLS
	clientOpts.SetTLSConfig(nil) // Uses default TLS config
	
	// Enable retry for writes and reads
	retryWrites := true
	retryReads := true
	clientOpts.SetRetryWrites(retryWrites)
	clientOpts.SetRetryReads(retryReads)
}

// validateEntraConfig validates the Entra authentication configuration
func validateEntraConfig(config *MongoAuthConfig) error {
	
	if len(config.Scopes) == 0 {
		config.Scopes = []string{"https://ossrdbms-aad.database.windows.net/.default"}
	}
	
	// Validate scopes for Azure Cosmos DB
	validScopeFound := false
	for _, scope := range config.Scopes {
		if scope == "https://ossrdbms-aad.database.windows.net/.default" {
			validScopeFound = true
			break
		}
		
		// Check for invalid scopes from other Azure services
		invalidScopes := []string{
			"https://database.windows.net/.default",              // SQL Server
			"https://vault.azure.net/.default",                   // Key Vault
			"https://storage.azure.com/.default",                 // Storage
		}
		
		for _, invalidScope := range invalidScopes {
			if scope == invalidScope {
				return fmt.Errorf("invalid scope for Azure Cosmos DB: %s", scope)
			}
		}
	}
	
	if !validScopeFound {
		return fmt.Errorf("invalid scope for Azure Cosmos DB, must include 'https://ossrdbms-aad.database.windows.net/.default'")
	}
	
	if config.RefreshBeforeExpiry == 0 {
		config.RefreshBeforeExpiry = 5 * time.Minute
	}
	
	return nil
}

// connectWithEntraAuth establishes a MongoDB connection using Azure Entra authentication
func connectWithEntraAuth(ctx context.Context, clientOpts *options.ClientOptions, config *MongoAuthConfig) (*mongo.Client, error) {
	// Initialize auth manager if needed
	if err := initAuthManager(config); err != nil {
		return nil, fmt.Errorf("failed to initialize auth manager: %w", err)
	}
	
	// Create OIDC credential with machine callback
	oidcCallback := createOIDCMachineCallback(config.TenantID, config.Scopes)
	
	// Set up MONGODB-OIDC authentication
	credential := options.Credential{
		AuthMechanism: "MONGODB-OIDC",
		OIDCMachineCallback: oidcCallback,
	}
	
	clientOpts.SetAuth(credential)
	
	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB with Entra auth: %w", err)
	}
	
	// Test the connection
	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(ctx)
		return nil, fmt.Errorf("failed to ping MongoDB with Entra auth: %w", err)
	}
	
	return client, nil
}

// initAuthManager initializes the global authentication manager
func initAuthManager(config *MongoAuthConfig) error {
	var initErr error
	
	authOnce.Do(func() {
		// Create Azure workload identity credential
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		// cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
		// 	TenantID: config.TenantID,
		// 	ClientID: config.ClientID,
		// })
		if err != nil {
			initErr = fmt.Errorf("failed to create workload identity credential: %w", err)
			return
		}
		
		authManager = &mongoAuthManager{
			credential:          cred,
			scopes:              config.Scopes,
			cache:               make(map[string]*tokenCacheEntry),
			refreshBeforeExpiry: config.RefreshBeforeExpiry,
		}
	})
	
	return initErr
}

// createOIDCMachineCallback creates the OIDC machine callback for MongoDB authentication
func createOIDCMachineCallback(tenantID string, scopes []string) func(context.Context, *options.OIDCArgs) (*options.OIDCCredential, error) {
	return func(ctx context.Context, args *options.OIDCArgs) (*options.OIDCCredential, error) {
		if authManager == nil {
			return nil, fmt.Errorf("auth manager not initialized")
		}
		
		// Create cache key based on tenant and scopes
		cacheKey := fmt.Sprintf("%s:%v", tenantID, scopes)
		
		// Use singleflight to prevent concurrent token requests
		tokenInterface, err, _ := authManager.group.Do(cacheKey, func() (interface{}, error) {
			return authManager.getOrRefreshToken(ctx, cacheKey, scopes)
		})
		
		if err != nil {
			return nil, fmt.Errorf("failed to get access token: %w", err)
		}
		
		token, ok := tokenInterface.(azcore.AccessToken)
		if !ok {
			return nil, fmt.Errorf("invalid token type")
		}
		
		return &options.OIDCCredential{
			AccessToken: token.Token,
			ExpiresAt:   &token.ExpiresOn,
		}, nil
	}
}

// getOrRefreshToken gets a cached token or refreshes it if expired/near expiry
func (m *mongoAuthManager) getOrRefreshToken(ctx context.Context, cacheKey string, scopes []string) (azcore.AccessToken, error) {
	m.cacheMutex.RLock()
	entry, exists := m.cache[cacheKey]
	m.cacheMutex.RUnlock()
	
	// Check if we have a valid cached token
	if exists {
		entry.mutex.RLock()
		token := entry.token
		expiresAt := entry.expiresAt
		entry.mutex.RUnlock()
		
		// Return cached token if it's still valid and not close to expiry
		if time.Now().Add(m.refreshBeforeExpiry).Before(expiresAt) {
			return token, nil
		}
	}
	
	// Get new token from Azure
	token, err := m.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: scopes,
	})
	if err != nil {
		return azcore.AccessToken{}, fmt.Errorf("failed to get token from Azure: %w", err)
	}
	
	// Cache the new token
	m.cacheMutex.Lock()
	if entry == nil {
		entry = &tokenCacheEntry{}
		m.cache[cacheKey] = entry
	}
	m.cacheMutex.Unlock()
	
	entry.mutex.Lock()
	entry.token = token
	entry.expiresAt = token.ExpiresOn
	entry.mutex.Unlock()
	
	return token, nil
}

// ClearTokenCache clears the token cache (useful for testing)
func ClearTokenCache() {
	if authManager != nil {
		authManager.cacheMutex.Lock()
		authManager.cache = make(map[string]*tokenCacheEntry)
		authManager.cacheMutex.Unlock()
	}
}

// GetCachedTokenCount returns the number of cached tokens (useful for testing)
func GetCachedTokenCount() int {
	if authManager == nil {
		return 0
	}
	
	authManager.cacheMutex.RLock()
	count := len(authManager.cache)
	authManager.cacheMutex.RUnlock()
	
	return count
}