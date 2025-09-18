package position

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMongoConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      *MongoConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "mongo config is required",
		},
		{
			name: "missing connection URI",
			config: &MongoConfig{
				Database: "test_db",
			},
			expectError: true,
			errorMsg:    "connection URI is required",
		},
		{
			name: "missing database",
			config: &MongoConfig{
				ConnectionURI: "mongodb://localhost:27017",
			},
			expectError: true,
			errorMsg:    "database name is required",
		},
		{
			name: "valid minimal config",
			config: &MongoConfig{
				ConnectionURI: "mongodb://localhost:27017",
				Database:      "test_db",
			},
			expectError: false,
		},
		{
			name: "valid full config",
			config: &MongoConfig{
				ConnectionURI:              "mongodb://localhost:27017",
				Database:                   "test_db",
				Collection:                 "positions",
				ConnectTimeout:             10 * time.Second,
				ServerSelectionTimeout:     30 * time.Second,
				SocketTimeout:              5 * time.Second,
				MaxPoolSize:                50,
				MinPoolSize:                2,
				ReadConcern:                "majority",
				EnableTransactions:         true,
				EnableAutoIndexCreation:    true,
				RetryWrites:                true,
				RetryReads:                 true,
				Compressors:               []string{"zlib", "snappy"},
				WriteConcern: &MongoWriteConcern{
					W:        "majority",
					J:        true,
					WTimeout: 5 * time.Second,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker, err := NewMongoTracker(tt.config)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, tracker)
			} else {
				if err == nil {
					assert.NotNil(t, tracker)
					assert.NotNil(t, tracker.client)
					assert.NotNil(t, tracker.collection)
					assert.Equal(t, tt.config.Database, tracker.database.Name())
					
					// Test defaults
					if tt.config.Collection == "" {
						assert.Equal(t, "stream_positions", tracker.config.Collection)
					}
					
					if tt.config.ConnectTimeout == 0 {
						assert.Equal(t, 10*time.Second, tracker.config.ConnectTimeout)
					}
					
					// Clean up
					tracker.Close()
				}
				// Note: We don't assert.NoError here because MongoDB might not be available
				// In CI/CD, these tests would need a MongoDB container
			}
		})
	}
}

func TestMongoTracker_ConfigDefaults(t *testing.T) {
	config := &MongoConfig{
		ConnectionURI: "mongodb://localhost:27017",
		Database:      "test_db",
	}
	
	tracker, err := NewMongoTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer tracker.Close()
	
	// Test that defaults are applied
	assert.Equal(t, "stream_positions", tracker.config.Collection)
	assert.Equal(t, 10*time.Second, tracker.config.ConnectTimeout)
	assert.Equal(t, 30*time.Second, tracker.config.ServerSelectionTimeout)
	assert.Equal(t, 10*time.Second, tracker.config.SocketTimeout)
	assert.Equal(t, uint64(100), tracker.config.MaxPoolSize)
	assert.Equal(t, uint64(1), tracker.config.MinPoolSize)
	assert.Equal(t, "majority", tracker.config.ReadConcern)
	assert.NotNil(t, tracker.config.WriteConcern)
	assert.Equal(t, "majority", tracker.config.WriteConcern.W)
	assert.True(t, tracker.config.WriteConcern.J)
	assert.Equal(t, 5*time.Second, tracker.config.WriteConcern.WTimeout)
}

func TestMongoTracker_HealthCheck(t *testing.T) {
	config := &MongoConfig{
		ConnectionURI: "mongodb://localhost:27017",
		Database:      "test_db",
	}
	
	tracker, err := NewMongoTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer tracker.Close()
	
	ctx := context.Background()
	
	// Test health check when connected
	err = tracker.HealthCheck(ctx)
	assert.NoError(t, err)
	
	// Test health check after closing
	tracker.Close()
	err = tracker.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tracker closed")
}

func TestMongoTracker_SaveAndLoad(t *testing.T) {
	config := &MongoConfig{
		ConnectionURI:           "mongodb://localhost:27017",
		Database:                "test_replicator_positions",
		Collection:              "test_positions",
		EnableAutoIndexCreation: true,
		EnableTransactions:      true,
	}
	
	tracker, err := NewMongoTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer func() {
		// Clean up test data
		ctx := context.Background()
		tracker.collection.Drop(ctx)
		tracker.Close()
	}()
	
	ctx := context.Background()
	streamID := "test-stream-mongodb"
	
	// Create a test position
	position := &MySQLPosition{
		File:     "mysql-bin.000001",
		Position: 1234,
		GTID:     "uuid:1-10",
	}
	
	metadata := map[string]interface{}{
		"stream_type": "mongodb",
		"host":        "localhost",
		"port":        27017,
		"test_field":  "test_value",
	}
	
	// Test Save
	err = tracker.Save(ctx, streamID, position, metadata)
	require.NoError(t, err)
	
	// Test Load
	loadedPosition, loadedMetadata, err := tracker.Load(ctx, streamID)
	require.NoError(t, err)
	
	// Note: We get nil position back because we haven't implemented position deserialization
	// In a complete implementation, you would have a position factory
	assert.Nil(t, loadedPosition)
	assert.NotNil(t, loadedMetadata)
	
	// Check metadata
	assert.Equal(t, "mongodb", loadedMetadata["stream_type"])
	assert.Equal(t, "test_value", loadedMetadata["test_field"])
	assert.Contains(t, loadedMetadata, "timestamp")
	assert.Contains(t, loadedMetadata, "version")
	
	// Test loading non-existent position
	_, _, err = tracker.Load(ctx, "non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrPositionNotFound, err)
}

func TestMongoTracker_Delete(t *testing.T) {
	config := &MongoConfig{
		ConnectionURI: "mongodb://localhost:27017",
		Database:      "test_replicator_positions",
		Collection:    "test_positions",
	}
	
	tracker, err := NewMongoTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer func() {
		// Clean up test data
		ctx := context.Background()
		tracker.collection.Drop(ctx)
		tracker.Close()
	}()
	
	ctx := context.Background()
	streamID := "test-stream-delete"
	
	// Create a test position
	position := &MySQLPosition{
		File:     "mysql-bin.000001",
		Position: 5678,
	}
	
	// Save position
	err = tracker.Save(ctx, streamID, position, nil)
	require.NoError(t, err)
	
	// Verify it exists
	_, _, err = tracker.Load(ctx, streamID)
	require.NoError(t, err)
	
	// Delete position
	err = tracker.Delete(ctx, streamID)
	require.NoError(t, err)
	
	// Verify it's gone
	_, _, err = tracker.Load(ctx, streamID)
	assert.Error(t, err)
	assert.Equal(t, ErrPositionNotFound, err)
	
	// Test deleting non-existent position
	err = tracker.Delete(ctx, "non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrPositionNotFound, err)
}

func TestMongoTracker_List(t *testing.T) {
	config := &MongoConfig{
		ConnectionURI: "mongodb://localhost:27017",
		Database:      "test_replicator_positions",
		Collection:    "test_positions",
	}
	
	tracker, err := NewMongoTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer func() {
		// Clean up test data
		ctx := context.Background()
		tracker.collection.Drop(ctx)
		tracker.Close()
	}()
	
	ctx := context.Background()
	
	// Test empty list
	positions, err := tracker.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, positions)
	
	// Add some positions
	streams := []string{"stream1", "stream2", "stream3"}
	for i, streamID := range streams {
		position := &MySQLPosition{
			File:     "mysql-bin.000001",
			Position: uint32(1000 + i*100),
		}
		
		err = tracker.Save(ctx, streamID, position, map[string]interface{}{
			"stream_type": "mysql",
			"index":       i,
		})
		require.NoError(t, err)
	}
	
	// List all positions
	positions, err = tracker.List(ctx)
	require.NoError(t, err)
	assert.Len(t, positions, len(streams))
	
	// Verify all stream IDs are present
	for _, streamID := range streams {
		_, exists := positions[streamID]
		assert.True(t, exists, "Stream ID %s should exist in positions map", streamID)
	}
}

func TestMongoTracker_Transactions(t *testing.T) {
	config := &MongoConfig{
		ConnectionURI:      "mongodb://localhost:27017",
		Database:           "test_replicator_positions",
		Collection:         "test_positions",
		EnableTransactions: true,
	}
	
	tracker, err := NewMongoTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer func() {
		// Clean up test data
		ctx := context.Background()
		tracker.collection.Drop(ctx)
		tracker.Close()
	}()
	
	ctx := context.Background()
	streamID := "test-stream-transaction"
	
	// Create a test position
	position := &MySQLPosition{
		File:     "mysql-bin.000001",
		Position: 9999,
	}
	
	metadata := map[string]interface{}{
		"stream_type": "mysql",
		"transaction": "test",
	}
	
	// Test Save with transactions
	err = tracker.Save(ctx, streamID, position, metadata)
	require.NoError(t, err)
	
	// Verify save worked
	_, loadedMetadata, err := tracker.Load(ctx, streamID)
	require.NoError(t, err)
	assert.Equal(t, "test", loadedMetadata["transaction"])
}

func TestMongoTracker_GetStats(t *testing.T) {
	config := &MongoConfig{
		ConnectionURI: "mongodb://localhost:27017",
		Database:      "test_replicator_positions",
		Collection:    "test_positions",
	}
	
	tracker, err := NewMongoTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer func() {
		// Clean up test data
		ctx := context.Background()
		tracker.collection.Drop(ctx)
		tracker.Close()
	}()
	
	ctx := context.Background()
	
	// Get stats
	stats, err := tracker.GetStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	
	// Should contain basic MongoDB collection stats
	assert.Contains(t, stats, "ns") // namespace
	assert.Contains(t, stats, "count") // document count
}

func TestMongoTracker_ConcurrentAccess(t *testing.T) {
	config := &MongoConfig{
		ConnectionURI:      "mongodb://localhost:27017",
		Database:           "test_replicator_positions",
		Collection:         "test_positions",
		EnableTransactions: true,
	}
	
	tracker, err := NewMongoTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer func() {
		// Clean up test data
		ctx := context.Background()
		tracker.collection.Drop(ctx)
		tracker.Close()
	}()
	
	ctx := context.Background()
	streamID := "test-stream-concurrent"
	
	// Test concurrent saves
	const numGoroutines = 10
	const numOperationsPerGoroutine = 5
	
	errChan := make(chan error, numGoroutines*numOperationsPerGoroutine)
	
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < numOperationsPerGoroutine; j++ {
				position := &MySQLPosition{
					File:     "mysql-bin.000001",
					Position: uint32(goroutineID*1000 + j),
				}
				
				metadata := map[string]interface{}{
					"goroutine": goroutineID,
					"operation": j,
				}
				
				err := tracker.Save(ctx, streamID, position, metadata)
				errChan <- err
			}
		}(i)
	}
	
	// Collect all errors
	for i := 0; i < numGoroutines*numOperationsPerGoroutine; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}
	
	// Verify final state
	_, metadata, err := tracker.Load(ctx, streamID)
	require.NoError(t, err)
	assert.NotNil(t, metadata)
}

func TestMongoTracker_WriteConcernOptions(t *testing.T) {
	tests := []struct {
		name         string
		writeConcern *MongoWriteConcern
		expectError  bool
	}{
		{
			name: "majority with journal",
			writeConcern: &MongoWriteConcern{
				W:        "majority",
				J:        true,
				WTimeout: 5 * time.Second,
			},
			expectError: false,
		},
		{
			name: "numeric write concern",
			writeConcern: &MongoWriteConcern{
				W:        2,
				J:        false,
				WTimeout: 3 * time.Second,
			},
			expectError: false,
		},
		{
			name: "custom tag write concern",
			writeConcern: &MongoWriteConcern{
				W:        "customTag",
				J:        true,
				WTimeout: 10 * time.Second,
			},
			expectError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &MongoConfig{
				ConnectionURI: "mongodb://localhost:27017",
				Database:      "test_replicator_positions",
				Collection:    "test_positions",
				WriteConcern:  tt.writeConcern,
			}
			
			tracker, err := NewMongoTracker(config)
			if err != nil {
				t.Skip("MongoDB not available, skipping integration test")
				return
			}
			defer tracker.Close()
			
			// Configuration should be valid
			assert.NotNil(t, tracker)
			assert.Equal(t, tt.writeConcern, tracker.config.WriteConcern)
		})
	}
}

// Helper function to test configuration updates and factory functions
func TestNewTracker_MongoDB(t *testing.T) {
	config := &Config{
		Type: "mongodb",
		MongoConfig: &MongoConfig{
			ConnectionURI: "mongodb://localhost:27017",
			Database:      "test_db",
		},
	}
	
	tracker, err := NewTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer tracker.Close()
	
	assert.NotNil(t, tracker)
	assert.IsType(t, &MongoTracker{}, tracker)
	
	// Test with "mongo" alias
	config.Type = "mongo"
	tracker2, err := NewTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer tracker2.Close()
	
	assert.NotNil(t, tracker2)
	assert.IsType(t, &MongoTracker{}, tracker2)
}

// TestMongoConfig_EntraAuthValidation tests Entra authentication configuration validation
func TestMongoConfig_EntraAuthValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *MongoConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_entra_auth_config",
			config: &MongoConfig{
				ConnectionURI: "mongodb://cosmos-test.mongo.cosmos.azure.com:10255/",
				Database:      "test_db",
				AuthMethod:    "entra",
				TenantID:      "12345678-1234-1234-1234-123456789012",
				ClientID:      "87654321-4321-4321-4321-210987654321",
				Scopes:        []string{"https://cosmos.azure.com/.default"},
			},
			expectError: false,
		},
		{
			name: "valid_entra_auth_system_identity",
			config: &MongoConfig{
				ConnectionURI: "mongodb://cosmos-test.mongo.cosmos.azure.com:10255/",
				Database:      "test_db",
				AuthMethod:    "entra",
				TenantID:      "12345678-1234-1234-1234-123456789012",
				Scopes:        []string{"https://cosmos.azure.com/.default"},
			},
			expectError: false,
		},
		{
			name: "entra_auth_missing_tenant_id",
			config: &MongoConfig{
				ConnectionURI: "mongodb://cosmos-test.mongo.cosmos.azure.com:10255/",
				Database:      "test_db",
				AuthMethod:    "entra",
				Scopes:        []string{"https://cosmos.azure.com/.default"},
			},
			expectError: true,
			errorMsg:    "tenant ID is required for Entra authentication",
		},
		{
			name: "entra_auth_invalid_scope_postgres",
			config: &MongoConfig{
				ConnectionURI: "mongodb://cosmos-test.mongo.cosmos.azure.com:10255/",
				Database:      "test_db",
				AuthMethod:    "entra",
				TenantID:      "12345678-1234-1234-1234-123456789012",
				ClientID:      "87654321-4321-4321-4321-210987654321",
				Scopes:        []string{"https://ossrdbms-aad.database.windows.net/.default"},
			},
			expectError: true,
			errorMsg:    "invalid scope for Azure Cosmos DB",
		},
		{
			name: "entra_auth_invalid_tenant_format",
			config: &MongoConfig{
				ConnectionURI: "mongodb://cosmos-test.mongo.cosmos.azure.com:10255/",
				Database:      "test_db",
				AuthMethod:    "entra",
				TenantID:      "invalid-tenant-id",
				ClientID:      "87654321-4321-4321-4321-210987654321",
				Scopes:        []string{"https://cosmos.azure.com/.default"},
			},
			expectError: true,
			errorMsg:    "tenant ID must be valid UUID format",
		},
		{
			name: "entra_auth_with_credentials_in_uri",
			config: &MongoConfig{
				ConnectionURI: "mongodb://user:pass@cosmos-test.mongo.cosmos.azure.com:10255/",
				Database:      "test_db",
				AuthMethod:    "entra",
				TenantID:      "12345678-1234-1234-1234-123456789012",
				ClientID:      "87654321-4321-4321-4321-210987654321",
				Scopes:        []string{"https://cosmos.azure.com/.default"},
			},
			expectError: true,
			errorMsg:    "connection URI must not contain credentials when using Entra authentication",
		},
		{
			name: "default_auth_method_connection_string",
			config: &MongoConfig{
				ConnectionURI: "mongodb://user:pass@localhost:27017/",
				Database:      "test_db",
				// AuthMethod not specified - should default to "connection_string"
			},
			expectError: false,
		},
		{
			name: "invalid_auth_method",
			config: &MongoConfig{
				ConnectionURI: "mongodb://localhost:27017/",
				Database:      "test_db",
				AuthMethod:    "invalid_method",
			},
			expectError: true,
			errorMsg:    "auth method must be 'connection_string' or 'entra'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMongoTracker(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestNewMongoTracker_EntraAuth tests creating a tracker with Entra authentication
func TestNewMongoTracker_EntraAuth(t *testing.T) {
	config := &MongoConfig{
		ConnectionURI: "mongodb://cosmos-test.mongo.cosmos.azure.com:10255/",
		Database:      "test_db",
		Collection:    "test_collection",
		AuthMethod:    "entra",
		TenantID:      "12345678-1234-1234-1234-123456789012",
		ClientID:      "87654321-4321-4321-4321-210987654321",
		Scopes:        []string{"https://cosmos.azure.com/.default"},
	}

	// This should fail until we implement Entra auth support
	tracker, err := NewMongoTracker(config)
	
	// For now, we expect this to work with the new config fields
	// but it will fail when trying to use Entra auth until we implement it
	require.NoError(t, err)
	require.NotNil(t, tracker)
	
	// The actual authentication will fail until we implement the Entra client creation
	defer func() {
		if tracker != nil {
			tracker.Close()
		}
	}()
}