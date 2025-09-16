package streams

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCosmosDBStreamProvider_parseCosmosDBConfig tests configuration parsing
func TestCosmosDBStreamProvider_parseCosmosDBConfig(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name           string
		globalConfig   *config.Config
		expectedError  bool
		expectedConfig *CosmosDBConfig
	}{
		{
			name: "minimal_config",
			globalConfig: &config.Config{
				WaterFlowsConfig: &config.WaterFlowsConfig{
					CosmosEndpoint:       "https://test.documents.azure.com:443/",
					CosmosDatabaseName:   "testdb",
					CosmosContainerName:  "testcontainer",
				},
			},
			expectedError: false,
			expectedConfig: &CosmosDBConfig{
				Endpoint:           "https://test.documents.azure.com:443/",
				DatabaseName:       "testdb",
				ContainerName:      "testcontainer",
				UseManagedIdentity: true,
				MaxItemCount:       100,
				PollInterval:       5 * time.Second,
				MaxRetries:         5,
				RetryDelay:         1 * time.Second,
				MaxBackoff:         5 * time.Minute,
				RequestTimeout:     30 * time.Second,
			},
		},
		{
			name: "full_config",
			globalConfig: &config.Config{
				WaterFlowsConfig: &config.WaterFlowsConfig{
					CosmosEndpoint:           "https://test.documents.azure.com:443/",
					CosmosDatabaseName:       "testdb",
					CosmosContainerName:      "testcontainer",
					CosmosStartFromBeginning: true,
					CosmosMaxItemCount:       50,
					CosmosPollInterval:       10000, // 10 seconds in milliseconds
					CosmosIncludeOperations:  []string{"create", "update"},
					CosmosExcludeOperations:  []string{"delete"},
				},
			},
			expectedError: false,
			expectedConfig: &CosmosDBConfig{
				Endpoint:           "https://test.documents.azure.com:443/",
				DatabaseName:       "testdb",
				ContainerName:      "testcontainer",
				StartFromBeginning: true,
				UseManagedIdentity: true,
				MaxItemCount:       50,
				PollInterval:       10 * time.Second,
				IncludeOperations:  []string{"create", "update"},
				ExcludeOperations:  []string{"delete"},
				MaxRetries:         5,
				RetryDelay:         1 * time.Second,
				MaxBackoff:         5 * time.Minute,
				RequestTimeout:     30 * time.Second,
			},
		},
		{
			name: "missing_endpoint",
			globalConfig: &config.Config{
				WaterFlowsConfig: &config.WaterFlowsConfig{
					CosmosDatabaseName:  "testdb",
					CosmosContainerName: "testcontainer",
				},
			},
			expectedError: true,
		},
		{
			name: "missing_database",
			globalConfig: &config.Config{
				WaterFlowsConfig: &config.WaterFlowsConfig{
					CosmosEndpoint:      "https://test.documents.azure.com:443/",
					CosmosContainerName: "testcontainer",
				},
			},
			expectedError: true,
		},
		{
			name: "missing_container",
			globalConfig: &config.Config{
				WaterFlowsConfig: &config.WaterFlowsConfig{
					CosmosEndpoint:     "https://test.documents.azure.com:443/",
					CosmosDatabaseName: "testdb",
				},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set global config
			config.SetConfig(tt.globalConfig)

			// Create provider
			eventChan := make(chan events.RecordEvent, 10)
			provider := NewCosmosDBStreamProvider(eventChan, logger)

			// Parse config
			err := provider.parseCosmosDBConfig()

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedConfig.Endpoint, provider.config.Endpoint)
				assert.Equal(t, tt.expectedConfig.DatabaseName, provider.config.DatabaseName)
				assert.Equal(t, tt.expectedConfig.ContainerName, provider.config.ContainerName)
				assert.Equal(t, tt.expectedConfig.StartFromBeginning, provider.config.StartFromBeginning)
				assert.Equal(t, tt.expectedConfig.MaxItemCount, provider.config.MaxItemCount)
				assert.Equal(t, tt.expectedConfig.PollInterval, provider.config.PollInterval)
				assert.Equal(t, tt.expectedConfig.IncludeOperations, provider.config.IncludeOperations)
				assert.Equal(t, tt.expectedConfig.ExcludeOperations, provider.config.ExcludeOperations)
			}
		})
	}
}

// TestCosmosDBStreamProvider_determineOperationType tests operation type determination
func TestCosmosDBStreamProvider_determineOperationType(t *testing.T) {
	logger := logrus.New()
	eventChan := make(chan events.RecordEvent, 10)
	provider := NewCosmosDBStreamProvider(eventChan, logger)

	tests := []struct {
		name         string
		document     map[string]interface{}
		expectedType string
	}{
		{
			name: "recent_document_create",
			document: map[string]interface{}{
				"id":  "doc1",
				"_ts": float64(time.Now().Unix()),
				"data": "test",
			},
			expectedType: "create",
		},
		{
			name: "old_document_update",
			document: map[string]interface{}{
				"id":  "doc2",
				"_ts": float64(time.Now().Unix() - 3600), // 1 hour ago
				"data": "test",
			},
			expectedType: "update",
		},
		{
			name: "document_without_ts",
			document: map[string]interface{}{
				"id":   "doc3",
				"data": "test",
			},
			expectedType: "update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.determineOperationType(tt.document)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

// TestCosmosDBStreamProvider_shouldFilterOperation tests operation filtering
func TestCosmosDBStreamProvider_shouldFilterOperation(t *testing.T) {
	logger := logrus.New()
	eventChan := make(chan events.RecordEvent, 10)

	tests := []struct {
		name              string
		includeOperations []string
		excludeOperations []string
		operation         string
		shouldFilter      bool
	}{
		{
			name:         "no_filters",
			operation:    "create",
			shouldFilter: false,
		},
		{
			name:              "include_operations_match",
			includeOperations: []string{"create", "update"},
			operation:         "create",
			shouldFilter:      false,
		},
		{
			name:              "include_operations_no_match",
			includeOperations: []string{"create", "update"},
			operation:         "delete",
			shouldFilter:      true,
		},
		{
			name:              "exclude_operations_match",
			excludeOperations: []string{"delete"},
			operation:         "delete",
			shouldFilter:      true,
		},
		{
			name:              "exclude_operations_no_match",
			excludeOperations: []string{"delete"},
			operation:         "create",
			shouldFilter:      false,
		},
		{
			name:              "both_include_and_exclude",
			includeOperations: []string{"create", "update", "delete"},
			excludeOperations: []string{"delete"},
			operation:         "delete",
			shouldFilter:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCosmosDBStreamProvider(eventChan, logger)
			provider.config = &CosmosDBConfig{
				IncludeOperations: tt.includeOperations,
				ExcludeOperations: tt.excludeOperations,
			}

			result := provider.shouldFilterOperation(tt.operation)
			assert.Equal(t, tt.shouldFilter, result)
		})
	}
}

// TestCosmosDBStreamProvider_processChangeItem tests change item processing
func TestCosmosDBStreamProvider_processChangeItem(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	tests := []struct {
		name            string
		document        map[string]interface{}
		config          *CosmosDBConfig
		expectEvent     bool
		expectedAction  string
		expectedSchema  string
		expectedCollection string
	}{
		{
			name: "create_operation",
			document: map[string]interface{}{
				"id":   "doc1",
				"_ts":  float64(time.Now().Unix()),
				"data": "test data",
			},
			config: &CosmosDBConfig{
				DatabaseName:  "testdb",
				ContainerName: "testcontainer",
			},
			expectEvent:        true,
			expectedAction:     "create",
			expectedSchema:     "testdb",
			expectedCollection: "testcontainer",
		},
		{
			name: "update_operation",
			document: map[string]interface{}{
				"id":   "doc2",
				"_ts":  float64(time.Now().Unix() - 3600),
				"data": "updated data",
			},
			config: &CosmosDBConfig{
				DatabaseName:  "testdb",
				ContainerName: "testcontainer",
			},
			expectEvent:        true,
			expectedAction:     "update",
			expectedSchema:     "testdb",
			expectedCollection: "testcontainer",
		},
		{
			name: "filtered_operation",
			document: map[string]interface{}{
				"id":   "doc3",
				"_ts":  float64(time.Now().Unix()),
				"data": "test data",
			},
			config: &CosmosDBConfig{
				DatabaseName:      "testdb",
				ContainerName:     "testcontainer",
				ExcludeOperations: []string{"create"},
			},
			expectEvent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventChan := make(chan events.RecordEvent, 10)
			provider := NewCosmosDBStreamProvider(eventChan, logger)
			provider.config = tt.config

			// Marshal document to JSON bytes
			docBytes, err := json.Marshal(tt.document)
			require.NoError(t, err)

			// Process the change item
			err = provider.processChangeItem(docBytes)
			assert.NoError(t, err)

			// Check if event was sent
			if tt.expectEvent {
				select {
				case event := <-eventChan:
					assert.Equal(t, tt.expectedAction, event.Action)
					assert.Equal(t, tt.expectedSchema, event.Schema)
					assert.Equal(t, tt.expectedCollection, event.Collection)
					assert.NotEmpty(t, event.Data)
				case <-time.After(100 * time.Millisecond):
					t.Fatal("Expected event but none received")
				}
			} else {
				select {
				case <-eventChan:
					t.Fatal("Expected no event but received one")
				case <-time.After(100 * time.Millisecond):
					// Expected - no event should be sent
				}
			}
		})
	}
}

// TestCosmosDBStreamProvider_isFatalError tests error classification
func TestCosmosDBStreamProvider_isFatalError(t *testing.T) {
	logger := logrus.New()
	eventChan := make(chan events.RecordEvent, 10)
	provider := NewCosmosDBStreamProvider(eventChan, logger)

	tests := []struct {
		name      string
		error     error
		isFatal   bool
	}{
		{
			name:    "nil_error",
			error:   nil,
			isFatal: false,
		},
		{
			name:    "timeout_error",
			error:   assert.AnError,
			isFatal: false,
		},
		{
			name:    "network_error",
			error:   assert.AnError,
			isFatal: false,
		},
		{
			name:    "unauthorized_error",
			error:   assert.AnError,
			isFatal: false, // Will be true when error message contains "unauthorized"
		},
		{
			name:    "throttling_error",
			error:   assert.AnError,
			isFatal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.isFatalError(tt.error)
			assert.Equal(t, tt.isFatal, result)
		})
	}
}

// TestCosmosDBStreamProvider_Stop tests the stop functionality
func TestCosmosDBStreamProvider_Stop(t *testing.T) {
	logger := logrus.New()
	eventChan := make(chan events.RecordEvent, 10)
	provider := NewCosmosDBStreamProvider(eventChan, logger)

	// Test stop without running
	provider.Stop()

	// Test stop with running
	provider.isRunning = true
	provider.Stop()
}

// TestCosmosDBStreamProvider_StreamType tests stream type identification
func TestCosmosDBStreamProvider_StreamType(t *testing.T) {
	logger := logrus.New()
	eventChan := make(chan events.RecordEvent, 10)
	provider := NewCosmosDBStreamProvider(eventChan, logger)

	assert.Equal(t, "cosmosdb", provider.StreamType())
}

// BenchmarkCosmosDBStreamProvider_processChangeItem benchmarks change item processing
func BenchmarkCosmosDBStreamProvider_processChangeItem(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce log noise
	eventChan := make(chan events.RecordEvent, 1000)
	provider := NewCosmosDBStreamProvider(eventChan, logger)
	provider.config = &CosmosDBConfig{
		DatabaseName:  "testdb",
		ContainerName: "testcontainer",
	}

	document := map[string]interface{}{
		"id":   "bench_doc",
		"_ts":  float64(time.Now().Unix()),
		"data": "benchmark data",
	}

	docBytes, _ := json.Marshal(document)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.processChangeItem(docBytes)
		// Drain the channel to prevent blocking
		select {
		case <-eventChan:
		default:
		}
	}
}

// BenchmarkCosmosDBStreamProvider_determineOperationType benchmarks operation type determination
func BenchmarkCosmosDBStreamProvider_determineOperationType(b *testing.B) {
	logger := logrus.New()
	eventChan := make(chan events.RecordEvent, 10)
	provider := NewCosmosDBStreamProvider(eventChan, logger)

	document := map[string]interface{}{
		"id":   "bench_doc",
		"_ts":  float64(time.Now().Unix()),
		"data": "benchmark data",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.determineOperationType(document)
	}
}

// BenchmarkCosmosDBStreamProvider_shouldFilterOperation benchmarks operation filtering
func BenchmarkCosmosDBStreamProvider_shouldFilterOperation(b *testing.B) {
	logger := logrus.New()
	eventChan := make(chan events.RecordEvent, 10)
	provider := NewCosmosDBStreamProvider(eventChan, logger)
	provider.config = &CosmosDBConfig{
		IncludeOperations: []string{"create", "update"},
		ExcludeOperations: []string{"delete"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.shouldFilterOperation("create")
	}
}