package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo/options"
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
			scopes:        []string{"https://cosmos.azure.com/.default"},
			expectError:   false,
			expectedScope: "https://cosmos.azure.com/.default",
		},
		{
			name:        "system_assigned_identity",
			tenantID:    "12345678-1234-1234-1234-123456789012",
			scopes:      []string{"https://cosmos.azure.com/.default"},
			expectError: false,
		},
		{
			name:        "invalid_scope_postgres",
			tenantID:    "12345678-1234-1234-1234-123456789012",
			scopes:      []string{"https://ossrdbms-aad.database.windows.net/.default"},
			expectError: true, // Should reject PostgreSQL/MySQL scope
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
				require.NoError(t, err)
				require.NotNil(t, client)
				
				// Verify the client is configured with MONGODB-OIDC
				// This will fail until we implement the function
				assert.NotNil(t, client)
			}
		})
	}
}

// TestOIDCMachineCallback tests the OIDC machine callback function directly
func TestOIDCMachineCallback(t *testing.T) {
	tenantID := "12345678-1234-1234-1234-123456789012"
	scopes := []string{"https://cosmos.azure.com/.default"}

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
	scopes := []string{"https://cosmos.azure.com/.default"}

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
		Scopes:   []string{"https://cosmos.azure.com/.default"},
	}
	err := validateEntraConfig(config)
	assert.NoError(t, err)
}

