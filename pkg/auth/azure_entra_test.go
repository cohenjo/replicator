package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultAzureEntraConfig(t *testing.T) {
	config := DefaultAzureEntraConfig()
	
	assert.NotEmpty(t, config.Authority)
	assert.NotEmpty(t, config.Scopes)
}

func TestNewAzureEntraProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  AzureEntraConfig
		wantErr bool
	}{
		{
			name: "valid config with client secret",
			config: AzureEntraConfig{
				TenantID:     "test-tenant",
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				Authority:    "https://login.microsoftonline.com/",
			},
			wantErr: false,
		},
		{
			name: "valid config with managed identity",
			config: AzureEntraConfig{
				TenantID:  "test-tenant",
				ClientID:  "test-client",
				Authority: "https://login.microsoftonline.com/",
			},
			wantErr: false,
		},
		{
			name: "invalid config - missing tenant",
			config: AzureEntraConfig{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
			},
			wantErr: true,
		},
		{
			name: "invalid config - no auth method",
			config: AzureEntraConfig{
				TenantID: "test-tenant",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewAzureEntraProvider(tt.config)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				assert.Equal(t, tt.config.TenantID, provider.config.TenantID)
			}
		})
	}
}

func TestNewAzureEntraAuthenticator(t *testing.T) {
	authenticator := NewAzureEntraAuthenticator()
	
	require.NotNil(t, authenticator)
	assert.NotNil(t, authenticator.providers)
}

func TestAzureEntraAuthenticatorValidateConfig(t *testing.T) {
	authenticator := NewAzureEntraAuthenticator()
	
	tests := []struct {
		name    string
		config  AuthConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: AuthConfig{
				Method: "azure_entra",
				Config: map[string]string{
					"tenant_id":     "test-tenant",
					"client_id":     "test-client",
					"client_secret": "test-secret",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid config - missing tenant",
			config: AuthConfig{
				Method: "azure_entra",
				Config: map[string]string{
					"client_id":     "test-client",
					"client_secret": "test-secret",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authenticator.ValidateConfig(tt.config)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAzureEntraAuthenticatorGetSupportedMethods(t *testing.T) {
	authenticator := NewAzureEntraAuthenticator()
	methods := authenticator.GetSupportedMethods()
	
	assert.Contains(t, methods, "azure_entra")
	assert.Contains(t, methods, "managed_identity")
	assert.Contains(t, methods, "service_principal")
	assert.Contains(t, methods, "client_credentials")
}

func TestValidateAzureEntraConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  AzureEntraConfig
		wantErr bool
	}{
		{
			name: "valid config with client secret",
			config: AzureEntraConfig{
				TenantID:     "test-tenant",
				ClientID:     "test-client",
				ClientSecret: "test-secret",
			},
			wantErr: false,
		},
		{
			name: "valid config with client ID only (managed identity)",
			config: AzureEntraConfig{
				TenantID: "test-tenant",
				ClientID: "test-client",
			},
			wantErr: false,
		},
		{
			name: "valid config with certificate",
			config: AzureEntraConfig{
				TenantID: "test-tenant",
				ClientID: "test-client",
				CertPath: "/path/to/cert",
			},
			wantErr: false,
		},
		{
			name: "invalid - missing tenant ID",
			config: AzureEntraConfig{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
			},
			wantErr: true,
		},
		{
			name: "invalid - no auth method",
			config: AzureEntraConfig{
				TenantID: "test-tenant",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAzureEntraConfig(tt.config)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConvertToAzureEntraConfig(t *testing.T) {
	tests := []struct {
		name       string
		authConfig AuthConfig
		wantErr    bool
		expected   AzureEntraConfig
	}{
		{
			name: "valid conversion",
			authConfig: AuthConfig{
				Method: "azure_entra",
				Config: map[string]string{
					"tenant_id":     "test-tenant",
					"client_id":     "test-client",
					"client_secret": "test-secret",
					"authority":     "https://login.microsoftonline.com/",
				},
			},
			wantErr: false,
			expected: AzureEntraConfig{
				TenantID:     "test-tenant",
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				Authority:    "https://login.microsoftonline.com/",
				Scopes:       []string{"https://graph.microsoft.com/.default"}, // Default value
			},
		},
		{
			name: "minimal valid conversion",
			authConfig: AuthConfig{
				Method: "azure_entra",
				Config: map[string]string{
					"tenant_id": "test-tenant",
					"client_id": "test-client",
				},
			},
			wantErr: false,
			expected: AzureEntraConfig{
				TenantID:  "test-tenant",
				ClientID:  "test-client",
				Authority: "https://login.microsoftonline.com/", // Default value from DefaultAzureEntraConfig
				Scopes:    []string{"https://graph.microsoft.com/.default"}, // Default value
			},
		},
		{
			name: "invalid conversion - missing tenant",
			authConfig: AuthConfig{
				Method: "azure_entra",
				Config: map[string]string{
					"client_id":     "test-client",
					"client_secret": "test-secret",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToAzureEntraConfig(tt.authConfig)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.TenantID, result.TenantID)
				assert.Equal(t, tt.expected.ClientID, result.ClientID)
				assert.Equal(t, tt.expected.ClientSecret, result.ClientSecret)
				assert.Equal(t, tt.expected.Authority, result.Authority)
				if len(tt.expected.Scopes) > 0 {
					assert.Equal(t, tt.expected.Scopes, result.Scopes)
				}
			}
		})
	}
}

func TestAzureEntraProviderCaching(t *testing.T) {
	config := AzureEntraConfig{
		TenantID: "test-tenant",
		ClientID: "test-client",
	}
	
	provider, err := NewAzureEntraProvider(config)
	require.NoError(t, err)
	require.NotNil(t, provider)
	
	// Test cache key generation
	scopes := []string{"scope1", "scope2"}
	key := provider.getCacheKey(scopes)
	assert.Contains(t, key, config.ClientID)
	
	// Test caching functionality
	credentials := &Credentials{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
		ExpiresIn:   3600,
		Scope:       "test-scope",
	}
	
	// Cache token
	provider.cacheToken(key, credentials, scopes)
	assert.Equal(t, 1, provider.GetCachedTokenCount())
	
	// Retrieve cached token
	cached := provider.getCachedToken(key)
	assert.NotNil(t, cached)
	assert.Equal(t, credentials.AccessToken, cached.Credentials.AccessToken)
	
	// Clear cache
	provider.ClearCache()
	assert.Equal(t, 0, provider.GetCachedTokenCount())
	
	// Check that token is no longer cached
	cached = provider.getCachedToken(key)
	assert.Nil(t, cached)
}

func TestAzureEntraProviderMetrics(t *testing.T) {
	config := AzureEntraConfig{
		TenantID: "test-tenant",
		ClientID: "test-client",
	}
	
	provider, err := NewAzureEntraProvider(config)
	require.NoError(t, err)
	require.NotNil(t, provider)
	
	// Initial metrics should be zero
	metrics := provider.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalAttempts)
	assert.Equal(t, int64(0), metrics.SuccessfulAuths)
	assert.Equal(t, int64(0), metrics.FailedAuths)
	assert.Equal(t, int64(0), metrics.TokensIssued)
	assert.Equal(t, float64(0), metrics.SuccessRate)
	
	// Simulate successful authentication
	provider.updateMetrics(true, time.Millisecond*100, true)
	
	metrics = provider.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalAttempts)
	assert.Equal(t, int64(1), metrics.SuccessfulAuths)
	assert.Equal(t, int64(0), metrics.FailedAuths)
	assert.Equal(t, int64(1), metrics.TokensIssued)
	assert.Equal(t, float64(1), metrics.SuccessRate)
	assert.NotNil(t, metrics.LastSuccessAt)
	assert.Nil(t, metrics.LastFailureAt)
	
	// Simulate failed authentication
	provider.updateMetrics(false, time.Millisecond*200, false)
	
	metrics = provider.GetMetrics()
	assert.Equal(t, int64(2), metrics.TotalAttempts)
	assert.Equal(t, int64(1), metrics.SuccessfulAuths)
	assert.Equal(t, int64(1), metrics.FailedAuths)
	assert.Equal(t, int64(1), metrics.TokensIssued)
	assert.Equal(t, float64(0.5), metrics.SuccessRate)
	assert.NotNil(t, metrics.LastSuccessAt)
	assert.NotNil(t, metrics.LastFailureAt)
}

func TestAzureEntraProviderValidateToken(t *testing.T) {
	config := AzureEntraConfig{
		TenantID: "test-tenant",
		ClientID: "test-client",
	}
	
	provider, err := NewAzureEntraProvider(config)
	require.NoError(t, err)
	require.NotNil(t, provider)
	
	ctx := context.Background()
	
	// Test with empty token
	result, err := provider.ValidateToken(ctx, "")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "EMPTY_TOKEN", result.ErrorCode)
	
	// Test with non-empty token
	result, err = provider.ValidateToken(ctx, "valid-token")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.NotNil(t, result.Credentials)
	assert.Equal(t, "valid-token", result.Credentials.AccessToken)
}

func TestAzureEntraProviderUnsupportedMethods(t *testing.T) {
	config := AzureEntraConfig{
		TenantID: "test-tenant",
		ClientID: "test-client",
	}
	
	provider, err := NewAzureEntraProvider(config)
	require.NoError(t, err)
	require.NotNil(t, provider)
	
	ctx := context.Background()
	
	// Test refresh token (should return error)
	_, err = provider.RefreshToken(ctx, "refresh-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
	
	// Test revoke token (should return error)
	err = provider.RevokeToken(ctx, "token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}