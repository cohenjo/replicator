package auth

import "errors"

// Authentication error definitions
var (
	ErrInvalidMethod       = errors.New("invalid authentication method")
	ErrInvalidTimeout      = errors.New("invalid timeout value")
	ErrInvalidMaxAttempts  = errors.New("invalid max attempts value")
	ErrInvalidInitialDelay = errors.New("invalid initial delay value")
	ErrInvalidMaxDelay     = errors.New("invalid max delay value")
	ErrInvalidMultiplier   = errors.New("invalid multiplier value")
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrTokenExpired        = errors.New("token expired")
	ErrTokenInvalid        = errors.New("token invalid")
	ErrProviderNotFound    = errors.New("authentication provider not found")
	ErrConfigurationInvalid = errors.New("authentication configuration invalid")
	ErrCredentialsNotFound  = errors.New("credentials not found")
	ErrRefreshFailed       = errors.New("token refresh failed")
	ErrRevokeFailed        = errors.New("token revoke failed")
)