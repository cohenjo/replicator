package auth

import (
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// AzureEntraProvider implements authentication using Azure Entra ID (formerly Azure AD)
type AzureEntraProvider struct {
	config          AzureEntraConfig
	credential      azcore.TokenCredential
	clientOptions   *azcore.ClientOptions
	tokenCache      map[string]*CachedToken
	cacheMutex      sync.RWMutex
	metrics         *AuthenticationMetrics
	metricsMutex    sync.RWMutex
}

// CachedToken represents a cached authentication token
type CachedToken struct {
	Credentials *Credentials  `json:"credentials"`
	CachedAt    time.Time     `json:"cached_at"`
	ExpiresAt   time.Time     `json:"expires_at"`
	Scopes      []string      `json:"scopes"`
}

// AzureEntraAuthenticator provides Azure Entra ID authentication services
type AzureEntraAuthenticator struct {
	providers map[string]*AzureEntraProvider
	mutex     sync.RWMutex
}

// NewAzureEntraProvider creates a new Azure Entra ID authentication provider
func NewAzureEntraProvider(config AzureEntraConfig) (*AzureEntraProvider, error) {
	if err := validateAzureEntraConfig(config); err != nil {
		return nil, fmt.Errorf("invalid Azure Entra configuration: %w", err)
	}

	provider := &AzureEntraProvider{
		config:        config,
		clientOptions: &azcore.ClientOptions{},
		tokenCache:    make(map[string]*CachedToken),
		metrics:       &AuthenticationMetrics{},
	}

	// Initialize credential based on configuration
	credential, err := provider.initializeCredential()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Azure credential: %w", err)
	}

	provider.credential = credential

	return provider, nil
}

// NewAzureEntraAuthenticator creates a new Azure Entra ID authenticator
func NewAzureEntraAuthenticator() *AzureEntraAuthenticator {
	return &AzureEntraAuthenticator{
		providers: make(map[string]*AzureEntraProvider),
	}
}

// initializeCredential initializes the appropriate Azure credential based on configuration
func (p *AzureEntraProvider) initializeCredential() (azcore.TokenCredential, error) {
	// Configure client options
	if p.config.Authority != "" {
		p.clientOptions.Cloud.ActiveDirectoryAuthorityHost = p.config.Authority
	}

	// Initialize credential based on available configuration
	if p.config.ClientSecret != "" {
		// Client secret credential (service principal)
		return azidentity.NewClientSecretCredential(
			p.config.TenantID,
			p.config.ClientID,
			p.config.ClientSecret,
			&azidentity.ClientSecretCredentialOptions{
				ClientOptions: *p.clientOptions,
			},
		)
	}

	if p.config.CertPath != "" {
		// Certificate credential
		certs, privKey, err := loadCertificateFromPath(p.config.CertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}

		return azidentity.NewClientCertificateCredential(
			p.config.TenantID,
			p.config.ClientID,
			certs,
			privKey,
			&azidentity.ClientCertificateCredentialOptions{
				ClientOptions: *p.clientOptions,
			},
		)
	}

	if p.config.ClientID != "" {
		// Managed identity with user-assigned identity
		return azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID:            azidentity.ClientID(p.config.ClientID),
			ClientOptions: *p.clientOptions,
		})
	}

	// Default to system-assigned managed identity
	return azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
		ClientOptions: *p.clientOptions,
	})
}

// GetToken obtains an authentication token
func (p *AzureEntraProvider) GetToken(ctx context.Context, scopes []string) (*Credentials, error) {
	startTime := time.Now()
	
	// Check cache first
	cacheKey := p.getCacheKey(scopes)
	if cachedToken := p.getCachedToken(cacheKey); cachedToken != nil {
		if !cachedToken.Credentials.IsExpired(5 * time.Minute) {
			p.updateMetrics(true, time.Since(startTime), false)
			return cachedToken.Credentials, nil
		}
		// Token expired, remove from cache
		p.removeCachedToken(cacheKey)
	}

	// Request new token
	tokenRequestOptions := policy.TokenRequestOptions{
		Scopes: scopes,
	}

	accessToken, err := p.credential.GetToken(ctx, tokenRequestOptions)
	if err != nil {
		p.updateMetrics(false, time.Since(startTime), false)
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Convert to our credentials format
	credentials := &Credentials{
		AccessToken: accessToken.Token,
		TokenType:   "Bearer",
		ExpiresAt:   accessToken.ExpiresOn,
		ExpiresIn:   int64(time.Until(accessToken.ExpiresOn).Seconds()),
		Scope:       fmt.Sprintf("%v", scopes),
	}

	// Cache the token
	p.cacheToken(cacheKey, credentials, scopes)
	
	p.updateMetrics(true, time.Since(startTime), true)
	return credentials, nil
}

// RefreshToken refreshes an existing token (not applicable for Azure Entra)
func (p *AzureEntraProvider) RefreshToken(ctx context.Context, refreshToken string) (*Credentials, error) {
	// Azure Entra ID tokens are refreshed automatically by the SDK
	// This method should not be used with Azure credentials
	return nil, errors.New("token refresh not supported for Azure Entra ID - tokens are automatically refreshed")
}

// ValidateToken validates a token
func (p *AzureEntraProvider) ValidateToken(ctx context.Context, token string) (*AuthenticationResult, error) {
	// For Azure tokens, we can attempt to use the token to make a simple API call
	// This is a basic validation - in production, you might want to validate against specific APIs
	
	result := &AuthenticationResult{
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Basic validation - check if token is not empty and has proper format
	if token == "" {
		result.Success = false
		result.Error = "token is empty"
		result.ErrorCode = "EMPTY_TOKEN"
		return result, nil
	}

	// Additional validation could be implemented here
	// For now, we'll assume the token is valid if it's not empty
	result.Success = true
	result.Credentials = &Credentials{
		AccessToken: token,
		TokenType:   "Bearer",
	}

	return result, nil
}

// RevokeToken revokes a token (not applicable for Azure Entra)
func (p *AzureEntraProvider) RevokeToken(ctx context.Context, token string) error {
	// Azure Entra ID doesn't support token revocation through the SDK
	// Tokens expire automatically
	return errors.New("token revocation not supported for Azure Entra ID - tokens expire automatically")
}

// Authenticate performs authentication using Azure Entra ID
func (a *AzureEntraAuthenticator) Authenticate(ctx context.Context, config AuthConfig) (*AuthenticationResult, error) {
	// Convert generic auth config to Azure Entra config
	azureConfig, err := convertToAzureEntraConfig(config)
	if err != nil {
		return &AuthenticationResult{
			Success:   false,
			Error:     fmt.Sprintf("invalid configuration: %v", err),
			ErrorCode: "INVALID_CONFIG",
			Timestamp: time.Now(),
		}, nil
	}

	// Get or create provider
	provider, err := a.getOrCreateProvider("default", azureConfig)
	if err != nil {
		return &AuthenticationResult{
			Success:   false,
			Error:     fmt.Sprintf("failed to create provider: %v", err),
			ErrorCode: "PROVIDER_CREATION_FAILED",
			Timestamp: time.Now(),
		}, nil
	}

	// Get token
	scopes := azureConfig.Scopes
	if len(scopes) == 0 {
		scopes = []string{"https://graph.microsoft.com/.default"}
	}

	credentials, err := provider.GetToken(ctx, scopes)
	if err != nil {
		return &AuthenticationResult{
			Success:   false,
			Error:     fmt.Sprintf("authentication failed: %v", err),
			ErrorCode: "AUTH_FAILED",
			Timestamp: time.Now(),
		}, nil
	}

	return &AuthenticationResult{
		Success:     true,
		Credentials: credentials,
		Timestamp:   time.Now(),
	}, nil
}

// GetProvider returns a token provider for the given configuration
func (a *AzureEntraAuthenticator) GetProvider(ctx context.Context, config AuthConfig) (TokenProvider, error) {
	azureConfig, err := convertToAzureEntraConfig(config)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return a.getOrCreateProvider("default", azureConfig)
}

// ValidateConfig validates the authentication configuration
func (a *AzureEntraAuthenticator) ValidateConfig(config AuthConfig) error {
	_, err := convertToAzureEntraConfig(config)
	return err
}

// GetSupportedMethods returns supported authentication methods
func (a *AzureEntraAuthenticator) GetSupportedMethods() []string {
	return []string{
		"azure_entra",
		"managed_identity",
		"service_principal",
		"client_credentials",
	}
}

// Helper methods

func (p *AzureEntraProvider) getCacheKey(scopes []string) string {
	return fmt.Sprintf("%s:%v", p.config.ClientID, scopes)
}

func (p *AzureEntraProvider) getCachedToken(key string) *CachedToken {
	p.cacheMutex.RLock()
	defer p.cacheMutex.RUnlock()
	
	token, exists := p.tokenCache[key]
	if !exists {
		return nil
	}
	
	// Check if token is still valid
	if time.Now().After(token.ExpiresAt) {
		return nil
	}
	
	return token
}

func (p *AzureEntraProvider) cacheToken(key string, credentials *Credentials, scopes []string) {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	
	p.tokenCache[key] = &CachedToken{
		Credentials: credentials,
		CachedAt:    time.Now(),
		ExpiresAt:   credentials.ExpiresAt,
		Scopes:      scopes,
	}
}

func (p *AzureEntraProvider) removeCachedToken(key string) {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	
	delete(p.tokenCache, key)
}

func (p *AzureEntraProvider) updateMetrics(success bool, duration time.Duration, tokenIssued bool) {
	p.metricsMutex.Lock()
	defer p.metricsMutex.Unlock()
	
	p.metrics.TotalAttempts++
	
	if success {
		p.metrics.SuccessfulAuths++
		now := time.Now()
		p.metrics.LastSuccessAt = &now
	} else {
		p.metrics.FailedAuths++
		now := time.Now()
		p.metrics.LastFailureAt = &now
	}
	
	if tokenIssued {
		p.metrics.TokensIssued++
	}
	
	// Update success rate
	if p.metrics.TotalAttempts > 0 {
		p.metrics.SuccessRate = float64(p.metrics.SuccessfulAuths) / float64(p.metrics.TotalAttempts)
	}
	
	// Update average latency (simple moving average)
	if p.metrics.TotalAttempts == 1 {
		p.metrics.AverageLatency = duration
	} else {
		total := p.metrics.AverageLatency * time.Duration(p.metrics.TotalAttempts-1)
		p.metrics.AverageLatency = (total + duration) / time.Duration(p.metrics.TotalAttempts)
	}
}

func (a *AzureEntraAuthenticator) getOrCreateProvider(name string, config AzureEntraConfig) (*AzureEntraProvider, error) {
	a.mutex.RLock()
	provider, exists := a.providers[name]
	a.mutex.RUnlock()
	
	if exists {
		return provider, nil
	}
	
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	// Double-check after acquiring write lock
	if provider, exists := a.providers[name]; exists {
		return provider, nil
	}
	
	// Create new provider
	newProvider, err := NewAzureEntraProvider(config)
	if err != nil {
		return nil, err
	}
	
	a.providers[name] = newProvider
	return newProvider, nil
}

// Validation and conversion functions

func validateAzureEntraConfig(config AzureEntraConfig) error {
	if config.TenantID == "" {
		return errors.New("tenant ID is required")
	}
	
	// At least one authentication method must be configured
	hasClientSecret := config.ClientSecret != ""
	hasCertificate := config.CertPath != ""
	hasClientID := config.ClientID != ""
	
	if !hasClientSecret && !hasCertificate && !hasClientID {
		return errors.New("at least one authentication method must be configured (client_secret, cert_path, or client_id for managed identity)")
	}
	
	return nil
}

func convertToAzureEntraConfig(config AuthConfig) (AzureEntraConfig, error) {
	azureConfig := DefaultAzureEntraConfig()
	
	// Extract values from generic config
	if tenantID, exists := config.Config["tenant_id"]; exists {
		azureConfig.TenantID = tenantID
	}
	
	if clientID, exists := config.Config["client_id"]; exists {
		azureConfig.ClientID = clientID
	}
	
	if clientSecret, exists := config.Config["client_secret"]; exists {
		azureConfig.ClientSecret = clientSecret
	}
	
	if certPath, exists := config.Config["cert_path"]; exists {
		azureConfig.CertPath = certPath
	}
	
	if authority, exists := config.Config["authority"]; exists {
		azureConfig.Authority = authority
	}
	
	// Validate the configuration
	if err := validateAzureEntraConfig(azureConfig); err != nil {
		return azureConfig, err
	}
	
	return azureConfig, nil
}

// loadCertificateFromPath loads certificate data from a file path
func loadCertificateFromPath(certPath string) ([]*x509.Certificate, crypto.PrivateKey, error) {
	// This is a placeholder implementation
	// In a real implementation, you would load the certificate and private key from the file
	return nil, nil, errors.New("certificate loading not implemented - placeholder")
}

// GetMetrics returns authentication metrics for the provider
func (p *AzureEntraProvider) GetMetrics() AuthenticationMetrics {
	p.metricsMutex.RLock()
	defer p.metricsMutex.RUnlock()
	
	// Return a copy of the metrics
	return *p.metrics
}

// ClearCache clears the token cache
func (p *AzureEntraProvider) ClearCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	
	p.tokenCache = make(map[string]*CachedToken)
}

// GetCachedTokenCount returns the number of cached tokens
func (p *AzureEntraProvider) GetCachedTokenCount() int {
	p.cacheMutex.RLock()
	defer p.cacheMutex.RUnlock()
	
	return len(p.tokenCache)
}