package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

// Loader handles configuration loading and validation
type Loader struct {
	validator *validator.Validate
}

// LoaderOptions represents options for the configuration loader
type LoaderOptions struct {
	// Environment variables prefix (e.g., "REPLICATOR_")
	EnvPrefix string
	// Default configuration file paths to search
	DefaultPaths []string
	// Whether to require configuration file to exist
	RequireFile bool
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{
		validator: validator.New(),
	}
}

// LoadFromFile loads configuration from a specific file
func (l *Loader) LoadFromFile(filename string) (*Config, error) {
	if filename == "" {
		return nil, fmt.Errorf("filename cannot be empty")
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", filename, err)
	}
	defer file.Close()

	return l.loadFromReader(file, filepath.Ext(filename))
}

// LoadFromReader loads configuration from an io.Reader
func (l *Loader) loadFromReader(reader io.Reader, fileExt string) (*Config, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read config data: %w", err)
	}

	// Start with default config to ensure all defaults are set
	config := DefaultConfig()

	switch strings.ToLower(fileExt) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", fileExt)
	}

	return config, nil
}

// Load loads configuration using the specified options
func (l *Loader) Load(opts LoaderOptions) (*Config, error) {
	var config *Config
	var err error

	// Try to load from environment variable first
	if configFile := os.Getenv(opts.EnvPrefix + "CONFIG_FILE"); configFile != "" {
		config, err = l.LoadFromFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config from env var specified file %s: %w", configFile, err)
		}
	} else {
		// Try default paths
		for _, path := range opts.DefaultPaths {
			if _, err := os.Stat(path); err == nil {
				config, err = l.LoadFromFile(path)
				if err != nil {
					return nil, fmt.Errorf("failed to load config from %s: %w", path, err)
				}
				break
			}
		}

		// If no config file found and required
		if config == nil && opts.RequireFile {
			return nil, fmt.Errorf("no configuration file found in paths: %v", opts.DefaultPaths)
		}

		// If no config file found but not required, create default
		if config == nil {
			config = DefaultConfig()
		}
	}

	// Override with environment variables
	if err := l.loadFromEnvironment(config, opts.EnvPrefix); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Validate configuration
	if err := l.Validate(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// LoadDefault loads configuration with sensible defaults
func (l *Loader) LoadDefault() (*Config, error) {
	return l.Load(LoaderOptions{
		EnvPrefix: "REPLICATOR_",
		DefaultPaths: []string{
			"./config.yaml",
			"./config.yml",
			"./config.json",
			"./conf/config.yaml",
			"./conf/config.yml",
			"./conf/config.json",
			"/etc/replicator/config.yaml",
			"/etc/replicator/config.yml",
			"/etc/replicator/config.json",
		},
		RequireFile: false,
	})
}

// Validate validates the configuration using struct tags
func (l *Loader) Validate(config *Config) error {
	if err := l.validator.Struct(config); err != nil {
		return l.formatValidationErrors(err)
	}

	// Custom validation logic
	if err := l.validateCustomRules(config); err != nil {
		return err
	}

	return nil
}

// loadFromEnvironment loads configuration values from environment variables
func (l *Loader) loadFromEnvironment(config *Config, prefix string) error {
	// Server configuration
	if port := os.Getenv(prefix + "SERVER_PORT"); port != "" {
		if portInt, err := strconv.Atoi(port); err == nil {
			config.Server.Port = portInt
		}
	}
	if host := os.Getenv(prefix + "SERVER_HOST"); host != "" {
		config.Server.Host = host
	}

	// Log level
	if logLevel := os.Getenv(prefix + "LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}

	// Metrics configuration
	if enabled := os.Getenv(prefix + "METRICS_ENABLED"); enabled != "" {
		config.Metrics.Enabled = strings.ToLower(enabled) == "true"
	}
	if port := os.Getenv(prefix + "METRICS_PORT"); port != "" {
		if portInt, err := strconv.Atoi(port); err == nil {
			config.Metrics.Port = portInt
		}
	}

	// Azure configuration
	if tenantID := os.Getenv(prefix + "AZURE_TENANT_ID"); tenantID != "" {
		config.Azure.Authentication.TenantID = tenantID
	}
	if clientID := os.Getenv(prefix + "AZURE_CLIENT_ID"); clientID != "" {
		config.Azure.Authentication.ClientID = clientID
	}
	if clientSecret := os.Getenv(prefix + "AZURE_CLIENT_SECRET"); clientSecret != "" {
		config.Azure.Authentication.ClientSecret = clientSecret
	}

	// Stream-specific environment variables
	l.loadStreamEnvironmentVariables(config, prefix)

	return nil
}

// loadStreamEnvironmentVariables loads stream-specific configuration from environment
func (l *Loader) loadStreamEnvironmentVariables(config *Config, prefix string) {
	// MongoDB connection
	if connStr := os.Getenv(prefix + "MONGODB_CONNECTION_STRING"); connStr != "" {
		// Find or create MongoDB stream and update connection string
		for i := range config.Streams {
			if config.Streams[i].Source.Type == SourceTypeMongoDB {
				config.Streams[i].Source.URI = connStr
			}
		}
	}

	// MySQL connection
	if connStr := os.Getenv(prefix + "MYSQL_CONNECTION_STRING"); connStr != "" {
		for i := range config.Streams {
			if config.Streams[i].Source.Type == SourceTypeMySQL {
				config.Streams[i].Source.URI = connStr
			}
		}
	}

	// PostgreSQL connection
	if connStr := os.Getenv(prefix + "POSTGRESQL_CONNECTION_STRING"); connStr != "" {
		for i := range config.Streams {
			if config.Streams[i].Source.Type == SourceTypePostgreSQL {
				config.Streams[i].Source.URI = connStr
			}
		}
	}

	// Cosmos DB connection
	if connStr := os.Getenv(prefix + "COSMOSDB_CONNECTION_STRING"); connStr != "" {
		for i := range config.Streams {
			if config.Streams[i].Source.Type == SourceTypeCosmosDB {
				config.Streams[i].Source.URI = connStr
			}
		}
	}
}

// validateCustomRules performs custom validation beyond struct tags
func (l *Loader) validateCustomRules(config *Config) error {
	// Validate stream configurations
	for i, stream := range config.Streams {
		if err := l.validateStream(stream, i); err != nil {
			return fmt.Errorf("stream %d validation failed: %w", i, err)
		}
	}

	// Validate Azure configuration if enabled
	if config.Azure.Authentication.Method != "" {
		if err := l.validateAzureConfig(config.Azure); err != nil {
			return fmt.Errorf("Azure configuration validation failed: %w", err)
		}
	}

	return nil
}

// validateAzureConfig validates Azure-specific configuration
func (l *Loader) validateAzureConfig(config AzureConfig) error {
	if config.Authentication.Method == "service_principal" {
		if config.Authentication.TenantID == "" {
			return fmt.Errorf("Azure tenant ID is required for service principal authentication")
		}
		if config.Authentication.ClientID == "" {
			return fmt.Errorf("Azure client ID is required for service principal authentication")
		}
		if config.Authentication.ClientSecret == "" {
			return fmt.Errorf("Azure client secret is required for service principal authentication")
		}
	}

	return nil
}

// NewDefaultConfig creates a default configuration
func NewDefaultConfig() *Config {
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
		Azure: AzureConfig{
			Authentication: AuthenticationConfig{
				Method: "managed_identity",
			},
		},
		Streams: []StreamConfig{},
	}
}

// validateStream validates individual stream configuration
func (l *Loader) validateStream(stream StreamConfig, index int) error {
	if stream.Name == "" {
		return fmt.Errorf("stream name cannot be empty")
	}

	// Validate source configuration
	switch stream.Source.Type {
	case SourceTypeMongoDB:
		if stream.Source.URI == "" && stream.Source.Host == "" {
			return fmt.Errorf("MongoDB connection string or host is required")
		}
		if stream.Source.Database == "" {
			return fmt.Errorf("MongoDB database is required")
		}
	case SourceTypeMySQL:
		if stream.Source.URI == "" && stream.Source.Host == "" {
			return fmt.Errorf("MySQL connection string or host is required")
		}
	case SourceTypePostgreSQL:
		if stream.Source.URI == "" && stream.Source.Host == "" {
			return fmt.Errorf("PostgreSQL connection string or host is required")
		}
	case SourceTypeCosmosDB:
		if stream.Source.URI == "" && stream.Source.Host == "" {
			return fmt.Errorf("Cosmos DB connection string or host is required")
		}
		if stream.Source.Database == "" {
			return fmt.Errorf("Cosmos DB database is required")
		}
	default:
		return fmt.Errorf("unsupported source type: %s", stream.Source.Type)
	}

	// Validate target configuration
	if err := l.validateTarget(stream.Target); err != nil {
		return fmt.Errorf("target validation failed: %w", err)
	}

	return nil
}

// validateTarget validates target configuration
func (l *Loader) validateTarget(target TargetConfig) error {
	switch target.Type {
	case TargetTypeKafka:
		if target.Host == "" && target.URI == "" {
			return fmt.Errorf("Kafka host or URI is required")
		}
	case TargetTypeElastic:
		if target.Host == "" && target.URI == "" {
			return fmt.Errorf("Elasticsearch host or URL is required")
		}
	case TargetTypeMongoDB:
		if target.URI == "" && target.Host == "" {
			return fmt.Errorf("MongoDB connection string or host is required")
		}
	default:
		return fmt.Errorf("unsupported target type: %s", target.Type)
	}

	return nil
}

// formatValidationErrors formats validator errors into a readable format
func (l *Loader) formatValidationErrors(err error) error {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var messages []string
		for _, validationError := range validationErrors {
			messages = append(messages, fmt.Sprintf(
				"field '%s' failed validation: %s",
				validationError.Field(),
				validationError.Tag(),
			))
		}
		return fmt.Errorf("validation errors: %s", strings.Join(messages, "; "))
	}
	return err
}

// SaveToFile saves configuration to a file
func (l *Loader) SaveToFile(config *Config, filename string) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create config file %s: %w", filename, err)
	}
	defer file.Close()

	ext := filepath.Ext(filename)
	switch strings.ToLower(ext) {
	case ".yaml", ".yml":
		encoder := yaml.NewEncoder(file)
		encoder.SetIndent(2)
		if err := encoder.Encode(config); err != nil {
			return fmt.Errorf("failed to encode YAML config: %w", err)
		}
	case ".json":
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(config); err != nil {
			return fmt.Errorf("failed to encode JSON config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}

	return nil
}

// GenerateTemplate generates a configuration template with all available options
func (l *Loader) GenerateTemplate() *Config {
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
		Azure: AzureConfig{
			Authentication: AuthenticationConfig{
				Method: "managed_identity",
			},
		},
		Streams: []StreamConfig{
			{
				Name:        "example-mongodb-stream",
				Enabled:     false,
				Source: SourceConfig{
					Type:     SourceTypeMongoDB,
					URI:      "mongodb://localhost:27017",
					Database: "mydb",
				},
				Target: TargetConfig{
					Type: TargetTypeKafka,
					Host: "localhost:9092",
				},
			},
			{
				Name:        "example-postgresql-stream",
				Enabled:     false,
				Source: SourceConfig{
					Type:     SourceTypePostgreSQL,
					URI:      "postgres://user:password@localhost:5432/mydb",
					Database: "mydb",
				},
				Target: TargetConfig{
					Type: TargetTypeElastic,
					Host: "localhost:9200",
				},
			},
		},
	}
}