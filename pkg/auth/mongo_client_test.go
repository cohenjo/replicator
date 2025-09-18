package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TestNewMongoClientWithAuth_OIDCCallback tests the MONGODB-OIDC credential callback
func TestNewMongoClientWithAuth_OIDCCallback(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		clientID       string
		scopes         []string
		expectError    bool
		expectedScope  string
	}{
		{
			name:          "valid_cosmos_db_config",
			tenantID:      "12345678-1234-1234-1234-123456789012",
			clientID:      "87654321-4321-4321-4321-210987654321",
			scopes:        []string{"https://ossrdbms-aad.database.windows.net/.default"},
			expectError:   false,
			expectedScope: "https://ossrdbms-aad.database.windows.net/.default",
		},
		{
			name:        "system_assigned_identity",
			tenantID:    "12345678-1234-1234-1234-123456789012",
			scopes:      []string{"https://ossrdbms-aad.database.windows.net/.default"},
			expectError: false,
		},
		{
			name:        "invalid_scope_postgres",
			tenantID:    "12345678-1234-1234-1234-123456789012",
			scopes:      []string{"https://cosmos.azure.com/.default"},
			expectError: true, // Should reject invalid scope
		},
		{
			name:        "missing_tenant_id",
			scopes:      []string{"https://cosmos.azure.com/.default"},
			expectError: true,
		},
		{
			name:        "empty_scopes",
			tenantID:    "12345678-1234-1234-1234-123456789012",
			scopes:      []string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &MongoAuthConfig{
				ConnectionURI:   "mongodb://cosmos-test.mongo.cosmos.azure.com:10255/",
				AuthMethod:      "entra",
				TenantID:        tt.tenantID,
				ClientID:        tt.clientID,
				Scopes:          tt.scopes,
			}

			client, err := NewMongoClientWithAuth(context.Background(), config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				// For valid configs, we expect validation to pass but connection might fail
				// since we're not connecting to a real server in tests
				if err != nil {
					// Check if it's a connection error (expected in tests)
					assert.Contains(t, err.Error(), "failed to connect to MongoDB with Entra auth")
				} else {
					// If connection succeeds, verify client is valid
					require.NotNil(t, client)
					client.Disconnect(context.Background())
				}
			}
		})
	}
}

// TestOIDCMachineCallback tests the OIDC machine callback function directly
func TestOIDCMachineCallback(t *testing.T) {
	tenantID := "12345678-1234-1234-1234-123456789012"
	scopes := []string{"https://ossrdbms-aad.database.windows.net/.default"}

	callback := createOIDCMachineCallback(tenantID, scopes)

	args := &options.OIDCArgs{
		Version: 1,
		IDPInfo: nil, // For machine flow
		RefreshToken: nil, // Always nil for machine flow
	}

	ctx := context.Background()
	credential, err := callback(ctx, args)

	// This should fail until we implement the callback
	require.NoError(t, err)
	require.NotNil(t, credential)
	assert.NotEmpty(t, credential.AccessToken)
	assert.NotNil(t, credential.ExpiresAt)
	assert.True(t, credential.ExpiresAt.After(time.Now()))
}

// TestOIDCCallbackConcurrency tests that concurrent callback calls don't cause race conditions
func TestOIDCCallbackConcurrency(t *testing.T) {
	tenantID := "12345678-1234-1234-1234-123456789012"
	scopes := []string{"https://ossrdbms-aad.database.windows.net/.default"}

	callback := createOIDCMachineCallback(tenantID, scopes)

	// Test concurrent calls to ensure singleflight works
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	args := &options.OIDCArgs{
		Version: 1,
	}

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := callback(context.Background(), args)
			results <- err
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		// All should succeed (or all fail consistently)
		// This will fail until we implement proper concurrency control
		assert.NoError(t, err)
	}
}

// TestValidateScopeRejectsInvalidScopes tests that invalid scopes are rejected
func TestValidateScopeRejectsInvalidScopes(t *testing.T) {
	invalidScopes := [][]string{
		{"https://ossrdbms-aad.database.windows.net/.default"}, // PostgreSQL scope
		{"https://database.windows.net/.default"},              // SQL Server scope
		{"https://vault.azure.net/.default"},                   // Key Vault scope
		{"https://storage.azure.com/.default"},                 // Storage scope
	}

	for _, scopes := range invalidScopes {
		t.Run(scopes[0], func(t *testing.T) {
			config := &MongoAuthConfig{
				TenantID: "12345678-1234-1234-1234-123456789012",
				ClientID: "87654321-4321-4321-4321-210987654321",
				Scopes:   scopes,
			}

			err := validateEntraConfig(config)
			assert.Error(t, err, "Should reject invalid scope: %s", scopes[0])
		})
	}

	// Valid scope should pass
	config := &MongoAuthConfig{
		TenantID: "12345678-1234-1234-1234-123456789012",
		ClientID: "87654321-4321-4321-4321-210987654321",
		Scopes:   []string{"https://ossrdbms-aad.database.windows.net/.default"},
	}
	err := validateEntraConfig(config)
	assert.NoError(t, err)
}

// TestNewMongoClientWithAuth_ConnectionString tests connection string authentication
func TestNewMongoClientWithAuth_ConnectionString(t *testing.T) {
	config := &MongoAuthConfig{
		ConnectionURI: "mongodb://localhost:27017/test",
		AuthMethod:    "connection_string",
	}

	client, err := NewMongoClientWithAuth(context.Background(), config)

	// We expect this to fail with a connection error since no MongoDB is running
	// but the important thing is that it validates the config correctly and tries to connect
	if err != nil {
		// Should be a connection error, not a config validation error
		assert.Contains(t, err.Error(), "failed to connect to MongoDB with connection string auth")
	} else {
		// If it somehow succeeds, clean up
		require.NotNil(t, client)
		client.Disconnect(context.Background())
	}
}

// TestValidateEntraConfig tests the validation logic without network calls
func TestValidateEntraConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *MongoAuthConfig
		expectError bool
	}{
		{
			name: "valid_config",
			config: &MongoAuthConfig{
				TenantID: "12345678-1234-1234-1234-123456789012",
				Scopes:   []string{"https://ossrdbms-aad.database.windows.net/.default"},
			},
			expectError: false,
		},
		{
			name: "empty_scopes_auto_filled",
			config: &MongoAuthConfig{
				TenantID: "12345678-1234-1234-1234-123456789012",
				Scopes:   []string{},
			},
			expectError: false,
		},
		{
			name: "invalid_scope",
			config: &MongoAuthConfig{
				TenantID: "12345678-1234-1234-1234-123456789012",
				Scopes:   []string{"https://cosmos.azure.com/.default"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEntraConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

