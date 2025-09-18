package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/rs/zerolog/log"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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
		// Connect using the URI which should contain authentication credentials
		log.Debug().Msg("Using connection string authentication")
		client, err := mongo.Connect(clientOpts)
		if err != nil {
			log.Error().Err(err).Msg("Failed to connect to MongoDB with connection string")
			return nil, fmt.Errorf("failed to connect to MongoDB with connection string auth: %w", err)
		}
		
		log.Debug().Msg("MongoDB connection established, attempting ping...")
		// Test the connection
		if err := client.Ping(ctx, nil); err != nil {
			log.Error().Err(err).Msg("MongoDB ping failed")
			client.Disconnect(ctx)
			return nil, fmt.Errorf("failed to ping MongoDB with connection string auth: %w", err)
		}
		log.Debug().Msg("MongoDB ping successful!")
		
		return client, nil
		
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
	log.Debug().Msg("Starting Entra authentication for MongoDB")
	log.Debug().Str("tenant_id", config.TenantID).Str("client_id", config.ClientID).Interface("scopes", config.Scopes).Msg("Entra authentication config")

	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get azure credentials: %w", err)
	}
	azureIdentityTokenCallback := func(_ context.Context,
		_ *options.OIDCArgs) (*options.OIDCCredential, error) {
		accessToken, err := credential.GetToken(ctx, policy.TokenRequestOptions{
			Scopes: []string{"https://ossrdbms-aad.database.windows.net/.default"},
		})
		if err != nil {
			return nil, err
		}
		return &options.OIDCCredential{
			AccessToken: accessToken.Token,
		}, nil
		}
	auth := options.Credential{
		AuthMechanism:       "MONGODB-OIDC",
		OIDCMachineCallback: azureIdentityTokenCallback,
	}
	clientOptions := options.Client().
		ApplyURI(config.ConnectionURI).
		SetConnectTimeout(2 * time.Minute).
		SetRetryWrites(true).
		SetTLSConfig(&tls.Config{}).
		SetAuth(auth)

	log.Debug().Msg("Attempting to connect to MongoDB...")
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to MongoDB")
		return nil, fmt.Errorf("failed to connect to MongoDB with Entra auth: %w", err)
	}

	log.Debug().Msg("Client created")
		
	// // Initialize auth manager if needed
	// if err := initAuthManager(config); err != nil {
	// 	fmt.Printf("[ERROR] Failed to initialize auth manager: %v\n", err)
	// 	return nil, fmt.Errorf("failed to initialize auth manager: %w", err)
	// }
	// fmt.Printf("[DEBUG] Auth manager initialized successfully\n")
	
	// // Create OIDC credential with machine callback
	// oidcCallback := createOIDCMachineCallback(config.TenantID, config.Scopes)
	// fmt.Printf("[DEBUG] OIDC callback created\n")
	
	// // Set up MONGODB-OIDC authentication
	// credential := options.Credential{
	// 	AuthMechanism: "MONGODB-OIDC",
	// 	OIDCMachineCallback: oidcCallback,
	// }
	
	// clientOpts.SetAuth(credential)
	// fmt.Printf("[DEBUG] MongoDB client options configured with MONGODB-OIDC\n")
	
	// // Connect to MongoDB
	// fmt.Printf("[DEBUG] Attempting to connect to MongoDB...\n")
	// client, err := mongo.Connect(ctx, clientOpts)
	// if err != nil {
	// 	fmt.Printf("[ERROR] Failed to connect to MongoDB: %v\n", err)
	// 	return nil, fmt.Errorf("failed to connect to MongoDB with Entra auth: %w", err)
	// }
	log.Debug().Msg("MongoDB connection established, attempting ping...")
	
	// Test the connection
	if err := client.Ping(ctx, nil); err != nil {
		log.Error().Err(err).Msg("MongoDB ping failed")
		client.Disconnect(ctx)
		return nil, fmt.Errorf("failed to ping MongoDB with Entra auth: %w", err)
	}
	log.Debug().Msg("MongoDB ping successful!")
	
	return client, nil
}

// initAuthManager initializes the global authentication manager
func initAuthManager(config *MongoAuthConfig) error {
	var initErr error
	
	authOnce.Do(func() {
		log.Debug().Msg("Initializing auth manager with NewDefaultAzureCredential")
		// Create Azure workload identity credential
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		// cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
		// 	TenantID: config.TenantID,
		// 	ClientID: config.ClientID,
		// })
		if err != nil {
			log.Error().Err(err).Msg("Failed to create DefaultAzureCredential")
			initErr = fmt.Errorf("failed to create workload identity credential: %w", err)
			return
		}
		log.Debug().Msg("DefaultAzureCredential created successfully")
		
		authManager = &mongoAuthManager{
			credential:          cred,
			scopes:              config.Scopes,
			cache:               make(map[string]*tokenCacheEntry),
			refreshBeforeExpiry: config.RefreshBeforeExpiry,
		}
		log.Debug().Interface("scopes", config.Scopes).Dur("refresh_buffer", config.RefreshBeforeExpiry).Msg("Auth manager initialized")
	})
	
	return initErr
}

// createOIDCMachineCallback creates the OIDC machine callback for MongoDB authentication
func createOIDCMachineCallback(tenantID string, scopes []string) func(context.Context, *options.OIDCArgs) (*options.OIDCCredential, error) {
	return func(ctx context.Context, args *options.OIDCArgs) (*options.OIDCCredential, error) {
		log.Debug().Msg("OIDC callback invoked by MongoDB driver")
		log.Debug().Interface("context", ctx).Msg("Request context")
		
		if authManager == nil {
			log.Error().Msg("Auth manager not initialized")
			return nil, fmt.Errorf("auth manager not initialized")
		}
		
		// Create cache key based on tenant and scopes
		cacheKey := fmt.Sprintf("%s:%v", tenantID, scopes)
		log.Debug().Str("cache_key", cacheKey).Msg("Using cache key")
		
		// Use singleflight to prevent concurrent token requests
		log.Debug().Msg("Requesting token via singleflight...")
		tokenInterface, err, _ := authManager.group.Do(cacheKey, func() (interface{}, error) {
			return authManager.getOrRefreshToken(ctx, cacheKey, scopes)
		})
		
		if err != nil {
			log.Error().Err(err).Msg("Failed to get access token")
			return nil, fmt.Errorf("failed to get access token: %w", err)
		}
		
		token, ok := tokenInterface.(azcore.AccessToken)
		if !ok {
			log.Error().Msg("Invalid token type received")
			return nil, fmt.Errorf("invalid token type")
		}
		
		log.Debug().Time("expires_at", token.ExpiresOn).Int("token_length", len(token.Token)).Msg("Token acquired successfully")
		
		return &options.OIDCCredential{
			AccessToken: token.Token,
			ExpiresAt:   &token.ExpiresOn,
		}, nil
	}
}

// getOrRefreshToken gets a cached token or refreshes it if expired/near expiry
func (m *mongoAuthManager) getOrRefreshToken(ctx context.Context, cacheKey string, scopes []string) (azcore.AccessToken, error) {
	log.Debug().Str("cache_key", cacheKey).Msg("getOrRefreshToken called")
	
	m.cacheMutex.RLock()
	entry, exists := m.cache[cacheKey]
	m.cacheMutex.RUnlock()
	
	// Check if we have a valid cached token
	if exists {
		log.Debug().Msg("Found cached token entry")
		entry.mutex.RLock()
		token := entry.token
		expiresAt := entry.expiresAt
		entry.mutex.RUnlock()
		
		// Return cached token if it's still valid and not close to expiry
		if time.Now().Add(m.refreshBeforeExpiry).Before(expiresAt) {
			log.Debug().Time("expires_at", expiresAt).Msg("Using cached token")
			return token, nil
		}
		log.Debug().Msg("Cached token is expired or near expiry, refreshing...")
	} else {
		log.Debug().Msg("No cached token found, acquiring new token")
	}
	
	// Get new token from Azure
	log.Debug().Interface("scopes", scopes).Msg("Requesting new token from Azure")
	token, err := m.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: scopes,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get token from Azure")
		return azcore.AccessToken{}, fmt.Errorf("failed to get token from Azure: %w", err)
	}
	
	log.Debug().Time("expires", token.ExpiresOn).Msg("Successfully acquired new token from Azure")
	
	// Cache the new token
	m.cacheMutex.Lock()
	if entry == nil {
		entry = &tokenCacheEntry{}
		m.cache[cacheKey] = entry
		log.Debug().Msg("Created new cache entry")
	} else {
		log.Debug().Msg("Updated existing cache entry")
	}
	m.cacheMutex.Unlock()
	
	entry.mutex.Lock()
	entry.token = token
	entry.expiresAt = token.ExpiresOn
	entry.mutex.Unlock()
	
	log.Debug().Msg("Token cached successfully")
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