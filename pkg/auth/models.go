package auth

import (
	"context"
	"time"
)

// AuthConfig represents the authentication configuration
type AuthConfig struct {
	Method   string            `json:"method" yaml:"method"`     // "azure_entra", "managed_identity", "service_principal", "client_credentials"
	Config   map[string]string `json:"config" yaml:"config"`    // Method-specific configuration
	Timeout  time.Duration     `json:"timeout" yaml:"timeout"`  // Authentication timeout
	Retry    RetryConfig       `json:"retry" yaml:"retry"`      // Retry configuration
}

// AzureEntraConfig represents Azure Entra ID authentication configuration
type AzureEntraConfig struct {
	TenantID     string `json:"tenant_id" yaml:"tenant_id"`
	ClientID     string `json:"client_id" yaml:"client_id"`
	ClientSecret string `json:"client_secret,omitempty" yaml:"client_secret,omitempty"` // For service principal
	CertPath     string `json:"cert_path,omitempty" yaml:"cert_path,omitempty"`         // For certificate auth
	Scopes       []string `json:"scopes" yaml:"scopes"`                                 // OAuth scopes
	Authority    string `json:"authority,omitempty" yaml:"authority,omitempty"`         // Custom authority URL
}

// ManagedIdentityConfig represents managed identity configuration
type ManagedIdentityConfig struct {
	ClientID   string `json:"client_id,omitempty" yaml:"client_id,omitempty"`     // User-assigned managed identity
	ResourceID string `json:"resource_id,omitempty" yaml:"resource_id,omitempty"` // Target resource ID
}

// RetryConfig represents retry configuration for authentication
type RetryConfig struct {
	MaxAttempts int           `json:"max_attempts" yaml:"max_attempts"`
	InitialDelay time.Duration `json:"initial_delay" yaml:"initial_delay"`
	MaxDelay     time.Duration `json:"max_delay" yaml:"max_delay"`
	Multiplier   float64       `json:"multiplier" yaml:"multiplier"`
}

// Credentials represents authentication credentials
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope,omitempty"`
}

// AuthenticationResult represents the result of an authentication attempt
type AuthenticationResult struct {
	Success     bool                   `json:"success"`
	Credentials *Credentials           `json:"credentials,omitempty"`
	Error       string                 `json:"error,omitempty"`
	ErrorCode   string                 `json:"error_code,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TokenProvider represents an interface for obtaining authentication tokens
type TokenProvider interface {
	// GetToken obtains an authentication token
	GetToken(ctx context.Context, scopes []string) (*Credentials, error)
	
	// RefreshToken refreshes an existing token
	RefreshToken(ctx context.Context, refreshToken string) (*Credentials, error)
	
	// ValidateToken validates a token
	ValidateToken(ctx context.Context, token string) (*AuthenticationResult, error)
	
	// RevokeToken revokes a token
	RevokeToken(ctx context.Context, token string) error
}

// Authenticator represents an interface for authentication operations
type Authenticator interface {
	// Authenticate performs authentication
	Authenticate(ctx context.Context, config AuthConfig) (*AuthenticationResult, error)
	
	// GetProvider returns a token provider for the given configuration
	GetProvider(ctx context.Context, config AuthConfig) (TokenProvider, error)
	
	// ValidateConfig validates the authentication configuration
	ValidateConfig(config AuthConfig) error
	
	// GetSupportedMethods returns supported authentication methods
	GetSupportedMethods() []string
}

// AuthenticationMetrics represents metrics for authentication operations
type AuthenticationMetrics struct {
	TotalAttempts    int64         `json:"total_attempts"`
	SuccessfulAuths  int64         `json:"successful_auths"`
	FailedAuths      int64         `json:"failed_auths"`
	SuccessRate      float64       `json:"success_rate"`
	AverageLatency   time.Duration `json:"average_latency"`
	TokensIssued     int64         `json:"tokens_issued"`
	TokensRefreshed  int64         `json:"tokens_refreshed"`
	TokensRevoked    int64         `json:"tokens_revoked"`
	LastSuccessAt    *time.Time    `json:"last_success_at,omitempty"`
	LastFailureAt    *time.Time    `json:"last_failure_at,omitempty"`
	ActiveTokens     int64         `json:"active_tokens"`
	ExpiringTokens   int64         `json:"expiring_tokens"` // Tokens expiring within 1 hour
}

// DefaultAuthConfig returns a default authentication configuration
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		Method:  "managed_identity",
		Config:  make(map[string]string),
		Timeout: 30 * time.Second,
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
		},
	}
}

// DefaultAzureEntraConfig returns a default Azure Entra configuration
func DefaultAzureEntraConfig() AzureEntraConfig {
	return AzureEntraConfig{
		Scopes:    []string{"https://graph.microsoft.com/.default"},
		Authority: "https://login.microsoftonline.com/",
	}
}

// IsExpired checks if credentials are expired or about to expire
func (c *Credentials) IsExpired(buffer time.Duration) bool {
	if buffer == 0 {
		buffer = 5 * time.Minute // Default 5-minute buffer
	}
	return time.Now().Add(buffer).After(c.ExpiresAt)
}

// TimeToExpiry returns the time until the credentials expire
func (c *Credentials) TimeToExpiry() time.Duration {
	return time.Until(c.ExpiresAt)
}

// Validate validates the authentication configuration
func (c *AuthConfig) Validate() error {
	if c.Method == "" {
		return ErrInvalidMethod
	}
	
	if c.Timeout <= 0 {
		return ErrInvalidTimeout
	}
	
	return c.Retry.Validate()
}

// Validate validates the retry configuration
func (r *RetryConfig) Validate() error {
	if r.MaxAttempts <= 0 {
		return ErrInvalidMaxAttempts
	}
	
	if r.InitialDelay <= 0 {
		return ErrInvalidInitialDelay
	}
	
	if r.MaxDelay < r.InitialDelay {
		return ErrInvalidMaxDelay
	}
	
	if r.Multiplier <= 1.0 {
		return ErrInvalidMultiplier
	}
	
	return nil
}