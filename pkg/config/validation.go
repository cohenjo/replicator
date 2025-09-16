package config

import (
	"encoding/json"
	"fmt"
)

// ValidateConfig validates the entire configuration
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate server config
	if err := ValidateServerConfig(&cfg.Server); err != nil {
		return fmt.Errorf("server config validation failed: %w", err)
	}

	// Validate streams
	if len(cfg.Streams) == 0 {
		return fmt.Errorf("at least one stream must be configured")
	}

	for i, stream := range cfg.Streams {
		if err := ValidateStreamConfig(&stream); err != nil {
			return fmt.Errorf("stream %d validation failed: %w", i, err)
		}
	}

	// Validate Azure config if present
	if err := ValidateAzureConfig(&cfg.Azure); err != nil {
		return fmt.Errorf("azure config validation failed: %w", err)
	}

	return nil
}

// ValidateServerConfig validates server configuration
func ValidateServerConfig(cfg *ServerConfig) error {
	if cfg == nil {
		return fmt.Errorf("server config cannot be nil")
	}

	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	if cfg.Host == "" {
		return fmt.Errorf("host is required")
	}

	return nil
}

// ValidateStreamConfig validates stream configuration
func ValidateStreamConfig(cfg *StreamConfig) error {
	if cfg == nil {
		return fmt.Errorf("stream config cannot be nil")
	}

	if cfg.Name == "" {
		return fmt.Errorf("stream name is required")
	}

	// Validate source config
	if err := ValidateSourceConfig(&cfg.Source); err != nil {
		return fmt.Errorf("source config validation failed: %w", err)
	}

	// Validate target config
	if err := ValidateTargetConfig(&cfg.Target); err != nil {
		return fmt.Errorf("target config validation failed: %w", err)
	}

	// Validate transformation rules if present
	if cfg.Transformation != nil {
		if err := ValidateTransformationRules(cfg.Transformation); err != nil {
			return fmt.Errorf("transformation rules validation failed: %w", err)
		}
	}

	return nil
}

// ValidateSourceConfig validates source configuration
func ValidateSourceConfig(cfg *SourceConfig) error {
	if cfg == nil {
		return fmt.Errorf("source config cannot be nil")
	}

	if cfg.URI == "" {
		return fmt.Errorf("uri is required")
	}

	if cfg.Database == "" {
		return fmt.Errorf("database is required")
	}

	// Type-specific validation
	switch cfg.Type {
	case SourceTypeMongoDB:
		// MongoDB specific validation can be added here
	case SourceTypeMySQL, SourceTypePostgreSQL:
		// SQL database specific validation can be added here
	case SourceTypeCosmosDB:
		// Cosmos DB specific validation can be added here
	default:
		return fmt.Errorf("unsupported source type: %s", cfg.Type)
	}

	return nil
}

// ValidateTargetConfig validates target configuration
func ValidateTargetConfig(cfg *TargetConfig) error {
	if cfg == nil {
		return fmt.Errorf("target config cannot be nil")
	}

	if cfg.URI == "" {
		return fmt.Errorf("uri is required")
	}

	// Type-specific validation
	switch cfg.Type {
	case TargetTypeMongoDB:
		if cfg.Database == "" {
			return fmt.Errorf("database is required for MongoDB target")
		}
	case TargetTypeElastic:
		if cfg.Database == "" {
			return fmt.Errorf("index is required for Elasticsearch target")
		}
	case TargetTypeMySQL, TargetTypePostgreSQL:
		if cfg.Database == "" {
			return fmt.Errorf("database is required for SQL target")
		}
	case TargetTypeCosmosDB:
		if cfg.Database == "" {
			return fmt.Errorf("database is required for Cosmos DB target")
		}
	default:
		return fmt.Errorf("unsupported target type: %s", cfg.Type)
	}

	return nil
}

// ValidateTransformationRules validates transformation rules configuration
func ValidateTransformationRules(cfg *TransformationRulesConfig) error {
	if cfg == nil {
		return fmt.Errorf("transformation rules config cannot be nil")
	}

	if !cfg.Enabled {
		return nil // Skip validation if disabled
	}

	for i, rule := range cfg.Rules {
		if err := ValidateTransformationRule(&rule); err != nil {
			return fmt.Errorf("rule %d (%s) validation failed: %w", i, rule.Name, err)
		}
	}

	return nil
}

// ValidateTransformationRule validates a single transformation rule
func ValidateTransformationRule(rule *TransformationRule) error {
	if rule == nil {
		return fmt.Errorf("transformation rule cannot be nil")
	}

	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}

	// Validate conditions
	for i, condition := range rule.Conditions {
		if err := ValidateCondition(&condition); err != nil {
			return fmt.Errorf("condition %d validation failed: %w", i, err)
		}
	}

	// Validate actions
	if len(rule.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}

	for i, action := range rule.Actions {
		if err := ValidateAction(&action); err != nil {
			return fmt.Errorf("action %d validation failed: %w", i, err)
		}
	}

	return nil
}

// ValidateCondition validates a transformation condition
func ValidateCondition(condition *Condition) error {
	if condition == nil {
		return fmt.Errorf("condition cannot be nil")
	}

	if condition.Field == "" {
		return fmt.Errorf("field is required")
	}

	if condition.Operator == "" {
		return fmt.Errorf("operator is required")
	}

	// Validate supported operators
	supportedOperators := []string{"eq", "ne", "gt", "lt", "gte", "lte", "contains", "exists", "in", "not_in"}
	validOperator := false
	for _, op := range supportedOperators {
		if condition.Operator == op {
			validOperator = true
			break
		}
	}

	if !validOperator {
		return fmt.Errorf("unsupported operator: %s", condition.Operator)
	}

	// For operators that require a value, ensure it's provided
	if condition.Operator != "exists" && condition.Value == nil {
		return fmt.Errorf("value is required for operator: %s", condition.Operator)
	}

	return nil
}

// ValidateAction validates a transformation action
func ValidateAction(action *Action) error {
	if action == nil {
		return fmt.Errorf("action cannot be nil")
	}

	if action.Type == "" {
		return fmt.Errorf("action type is required")
	}

	// Validate supported action types
	supportedTypes := []string{"kazaam", "jq", "lua", "javascript"}
	validType := false
	for _, t := range supportedTypes {
		if action.Type == t {
			validType = true
			break
		}
	}

	if !validType {
		return fmt.Errorf("unsupported action type: %s", action.Type)
	}

	if action.Spec == "" {
		return fmt.Errorf("action spec is required")
	}

	// Validate JSON spec for kazaam actions
	if action.Type == "kazaam" {
		var spec interface{}
		if err := json.Unmarshal([]byte(action.Spec), &spec); err != nil {
			return fmt.Errorf("invalid JSON spec for kazaam action: %w", err)
		}
	}

	return nil
}

// ValidateAzureConfig validates Azure configuration
func ValidateAzureConfig(cfg *AzureConfig) error {
	if cfg == nil {
		// Azure config is optional
		return nil
	}

	// Validate authentication if present
	if err := ValidateAuthenticationConfig(&cfg.Authentication); err != nil {
		return fmt.Errorf("authentication config validation failed: %w", err)
	}

	return nil
}

// ValidateAuthenticationConfig validates Azure authentication configuration
func ValidateAuthenticationConfig(cfg *AuthenticationConfig) error {
	if cfg == nil {
		return nil // Authentication config is optional
	}

	if cfg.Method == "" {
		return nil // Method is optional, defaults will be used
	}

	validMethods := []string{"service_principal", "managed_identity", "cli"}
	validMethod := false
	for _, method := range validMethods {
		if cfg.Method == method {
			validMethod = true
			break
		}
	}

	if !validMethod {
		return fmt.Errorf("unsupported authentication method: %s", cfg.Method)
	}

	// Validate based on method
	switch cfg.Method {
	case "service_principal":
		if cfg.TenantID == "" {
			return fmt.Errorf("tenant_id is required for service principal authentication")
		}
		if cfg.ClientID == "" {
			return fmt.Errorf("client_id is required for service principal authentication")
		}
		if cfg.ClientSecret == "" && cfg.CertificatePath == "" {
			return fmt.Errorf("either client_secret or certificate_path is required for service principal authentication")
		}
	case "managed_identity":
		// Client ID is optional for managed identity
	case "cli":
		// No additional validation required for CLI authentication
	}

	return nil
}

// CreateDefaultConfig creates a new configuration with default values
func CreateDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Port:    9090,
			Path:    "/metrics",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Streams: []StreamConfig{},
	}
}

// ToYAML serializes the config to YAML
func ToYAML(cfg *Config) ([]byte, error) {
	return nil, fmt.Errorf("YAML serialization not implemented yet")
}

// FromYAML deserializes YAML to config
func FromYAML(data []byte, cfg *Config) error {
	return fmt.Errorf("YAML deserialization not implemented yet")
}