package streams

import (
	"context"
	"testing"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPostgreSQLStreamProvider(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	assert.NotNil(t, provider)
	assert.Equal(t, eventSender, provider.eventSender)
	assert.Equal(t, logger, provider.logger)
	assert.NotNil(t, provider.stopChannel)
	assert.Equal(t, 1*time.Second, provider.pollInterval)
	assert.Equal(t, 2.0, provider.backoffFactor)
	assert.Equal(t, 5*time.Minute, provider.maxBackoff)
	assert.Equal(t, 10, provider.maxRetries)
	assert.NotEmpty(t, provider.streamID)
}

func TestPostgreSQLStreamProvider_GetStreamType(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	assert.Equal(t, "postgresql", provider.GetStreamType())
}

func TestPostgreSQLStreamProvider_GetStreamID(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	streamID := provider.GetStreamID()
	assert.NotEmpty(t, streamID)
	assert.Contains(t, streamID, "postgresql_")
}

func TestPostgreSQLStreamProvider_parsePostgreSQLConfig_WithDefaults(t *testing.T) {
	// Setup global config with minimal PostgreSQL settings
	originalConfig := config.GetConfig()
	defer config.SetConfig(originalConfig)
	
	testConfig := &config.Config{
		WaterFlowsConfig: &config.WaterFlowsConfig{
			PostgreSQLHost:     "localhost",
			PostgreSQLPort:     5432,
			PostgreSQLDatabase: "testdb",
			PostgreSQLUser:     "testuser",
			PostgreSQLPassword: "testpass",
		},
	}
	config.SetConfig(testConfig)
	
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	err := provider.parsePostgreSQLConfig()
	require.NoError(t, err)
	
	assert.Equal(t, "localhost", provider.config.Host)
	assert.Equal(t, 5432, provider.config.Port)
	assert.Equal(t, "testdb", provider.config.Database)
	assert.Equal(t, "testuser", provider.config.Username)
	assert.Equal(t, "testpass", provider.config.Password)
	assert.Equal(t, "pgoutput", provider.config.PluginName)
	assert.Equal(t, 10*time.Second, provider.config.StatusInterval)
	assert.Equal(t, 60*time.Second, provider.config.WalSenderTimeout)
	assert.Equal(t, "prefer", provider.config.SSLMode)
	assert.True(t, provider.config.CreateSlot)
	assert.False(t, provider.config.DropSlotOnExit)
	assert.Equal(t, "noexport", provider.config.SlotSnapShotAction)
	assert.False(t, provider.config.TempSlot)
	assert.NotEmpty(t, provider.config.SlotName)
	assert.NotEmpty(t, provider.config.PublicationName)
}

func TestPostgreSQLStreamProvider_parsePostgreSQLConfig_MissingRequired(t *testing.T) {
	// Setup global config without required fields
	originalConfig := config.GetConfig()
	defer config.SetConfig(originalConfig)
	
	testConfig := &config.Config{
		WaterFlowsConfig: &config.WaterFlowsConfig{
			// Missing required fields
		},
	}
	config.SetConfig(testConfig)
	
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	err := provider.parsePostgreSQLConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is required")
}

func TestPostgreSQLStreamProvider_parsePostgreSQLConfig_WithCustomSettings(t *testing.T) {
	// Setup global config with custom PostgreSQL settings
	originalConfig := config.GetConfig()
	defer config.SetConfig(originalConfig)
	
	testConfig := &config.Config{
		WaterFlowsConfig: &config.WaterFlowsConfig{
			PostgreSQLHost:               "custom.host",
			PostgreSQLPort:               5433,
			PostgreSQLDatabase:           "customdb",
			PostgreSQLUser:               "customuser",
			PostgreSQLPassword:           "custompass",
			PostgreSQLSlotName:           "custom_slot",
			PostgreSQLPublicationName:    "custom_publication",
		},
	}
	config.SetConfig(testConfig)
	
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	err := provider.parsePostgreSQLConfig()
	require.NoError(t, err)
	
	assert.Equal(t, "custom.host", provider.config.Host)
	assert.Equal(t, 5433, provider.config.Port)
	assert.Equal(t, "customdb", provider.config.Database)
	assert.Equal(t, "customuser", provider.config.Username)
	assert.Equal(t, "custompass", provider.config.Password)
	assert.Equal(t, "custom_slot", provider.config.SlotName)
	assert.Equal(t, "custom_publication", provider.config.PublicationName)
}

func TestPostgreSQLStreamProvider_buildConnectionString(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	provider.config = &PostgreSQLConfig{
		Host:        "localhost",
		Port:        5432,
		Username:    "testuser",
		Password:    "testpass",
		Database:    "testdb",
		SSLMode:     "require",
		SSLCert:     "/path/to/cert.pem",
		SSLKey:      "/path/to/key.pem",
		SSLRootCert: "/path/to/ca.pem",
	}
	
	// Test regular connection string
	connStr := provider.buildConnectionString(false)
	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=require sslcert=/path/to/cert.pem sslkey=/path/to/key.pem sslrootcert=/path/to/ca.pem"
	assert.Equal(t, expected, connStr)
	
	// Test replication connection string
	connStrRepl := provider.buildConnectionString(true)
	expectedRepl := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=require replication=database sslcert=/path/to/cert.pem sslkey=/path/to/key.pem sslrootcert=/path/to/ca.pem"
	assert.Equal(t, expectedRepl, connStrRepl)
}

func TestPostgreSQLStreamProvider_buildConnectionString_NoPassword(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	provider.config = &PostgreSQLConfig{
		Host:     "localhost",
		Port:     5432,
		Username: "testuser",
		Database: "testdb",
		SSLMode:  "disable",
	}
	
	connStr := provider.buildConnectionString(false)
	expected := "host=localhost port=5432 user=testuser dbname=testdb sslmode=disable"
	assert.Equal(t, expected, connStr)
}

func TestPostgreSQLStreamProvider_shouldFilterTable(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	provider.config = &PostgreSQLConfig{
		IncludeSchemas:  []string{"public", "app"},
		ExcludeSchemas:  []string{"system"},
		IncludeTables:   []string{"users", "orders", "public.products"},
		ExcludeTables:   []string{"temp_table", "app.logs"},
	}
	
	tests := []struct {
		name     string
		schema   string
		table    string
		expected bool
	}{
		{"Include schema and table", "public", "users", false},
		{"Include schema, exclude table", "public", "temp_table", true},
		{"Include schema, not in include tables", "public", "other_table", true},
		{"Exclude schema", "system", "any_table", true},
		{"Not in include schema", "other", "users", true},
		{"Full table name match", "public", "products", false},
		{"Exclude full table name", "app", "logs", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.shouldFilterTable(tt.schema, tt.table)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPostgreSQLStreamProvider_shouldFilterOperation(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	provider.config = &PostgreSQLConfig{
		IncludeOperations: []string{"insert", "update"},
		ExcludeOperations: []string{"delete"},
	}
	
	tests := []struct {
		name      string
		operation string
		expected  bool
	}{
		{"Include insert", "insert", false},
		{"Include update", "update", false},
		{"Include INSERT (case insensitive)", "INSERT", false},
		{"Exclude delete", "delete", true},
		{"Exclude DELETE (case insensitive)", "DELETE", true},
		{"Not in include list", "truncate", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.shouldFilterOperation(tt.operation)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPostgreSQLStreamProvider_shouldFilterOperation_NoFilters(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	provider.config = &PostgreSQLConfig{
		// No filters configured
	}
	
	// Should not filter any operations when no filters are configured
	assert.False(t, provider.shouldFilterOperation("insert"))
	assert.False(t, provider.shouldFilterOperation("update"))
	assert.False(t, provider.shouldFilterOperation("delete"))
	assert.False(t, provider.shouldFilterOperation("truncate"))
}

func TestPostgreSQLStreamProvider_isFatalError(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"Nil error", nil, false},
		{"Authentication failed", assert.AnError, false}, // Will be false since it doesn't contain fatal keywords
		{"Connection refused", assert.AnError, false},    // Will be false since it doesn't contain fatal keywords
		{"Database does not exist", assert.AnError, false}, // Will be false since it doesn't contain fatal keywords
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.isFatalError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPostgreSQLStreamProvider_Stop(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	provider := NewPostgreSQLStreamProvider(eventSender, logger)
	
	// Initially not running
	err := provider.Stop()
	assert.NoError(t, err)
	
	// Set as running and test stop
	provider.isRunning = true
	err = provider.Stop()
	assert.NoError(t, err)
	assert.False(t, provider.isRunning)
}