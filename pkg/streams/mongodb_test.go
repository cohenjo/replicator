package streams

import (
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
)

func TestNewMongoDBStreamProvider(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.WaterFlowsConfig
		expected string
	}{
		{
			name: "valid_config",
			config: &config.WaterFlowsConfig{
				Type:       "mongodb",
				Host:       "localhost",
				Port:       27017,
				Schema:     "testdb",
				Collection: "testcoll",
			},
			expected: "MongoDB",
		},
	}

	// Create a channel for events
	eventChan := make(chan *events.RecordEvent, 100)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMongoDBStreamProvider(&eventChan, tt.config)
			assert.NotNil(t, provider)
			assert.Equal(t, tt.expected, provider.StreamType())
			assert.Equal(t, &eventChan, provider.events)
			assert.Equal(t, tt.config, provider.config)
			assert.NotNil(t, provider.stopChan)
		})
	}
}

func TestMongoDBStreamProvider_parseMongoDBConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.WaterFlowsConfig
		expectedConfig *MongoDBStreamConfig
	}{
		{
			name: "minimal_config",
			config: &config.WaterFlowsConfig{
				Type:       "mongodb",
				Host:       "localhost",
				Port:       27017,
				Schema:     "testdb",
				Collection: "testcoll",
			},
			expectedConfig: &MongoDBStreamConfig{
				Database:        "testdb",
				Collection:      "testcoll",
				WatchCollection: true,
				WatchDatabase:   false,
				FullDocument:    "updateLookup",
				BatchSize:       1000,
				MaxAwaitTime:    5 * time.Second,
				UseReplicaSet:   true,
				ReadPreference:  "primary",
			},
		},
		{
			name: "database_only_config",
			config: &config.WaterFlowsConfig{
				Type:   "mongodb",
				Host:   "localhost",
				Port:   27017,
				Schema: "testdb",
			},
			expectedConfig: &MongoDBStreamConfig{
				Database:        "testdb",
				Collection:      "",
				WatchCollection: false,
				WatchDatabase:   true,
				FullDocument:    "updateLookup",
				BatchSize:       1000,
				MaxAwaitTime:    5 * time.Second,
				UseReplicaSet:   true,
				ReadPreference:  "primary",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &MongoDBStreamProvider{config: tt.config}
			result, err := provider.parseMongoDBConfig()
			require.NoError(t, err)

			assert.Equal(t, tt.expectedConfig.Database, result.Database)
			assert.Equal(t, tt.expectedConfig.Collection, result.Collection)
			assert.Equal(t, tt.expectedConfig.WatchCollection, result.WatchCollection)
			assert.Equal(t, tt.expectedConfig.WatchDatabase, result.WatchDatabase)
			assert.Equal(t, tt.expectedConfig.FullDocument, result.FullDocument)
			assert.Equal(t, tt.expectedConfig.BatchSize, result.BatchSize)
			assert.Equal(t, tt.expectedConfig.MaxAwaitTime, result.MaxAwaitTime)
			assert.Equal(t, tt.expectedConfig.ReadPreference, result.ReadPreference)
		})
	}
}

func TestMongoDBStreamProvider_createChangeStreamOptions(t *testing.T) {
	provider := &MongoDBStreamProvider{}
	config := &MongoDBStreamConfig{
		FullDocument: "updateLookup",
		BatchSize:    500,
		MaxAwaitTime: 10 * time.Second,
		StartAtOperationTime: &time.Time{},
	}

	opts := provider.createChangeStreamOptions(config)
	assert.NotNil(t, opts)
	// Note: We can't easily test the internal state of options.ChangeStreamOptions
	// This test mainly ensures the function doesn't panic
}

func TestMongoDBStreamProvider_createPipeline(t *testing.T) {
	provider := &MongoDBStreamProvider{}
	
	tests := []struct {
		name           string
		config         *MongoDBStreamConfig
		expectedStages int
	}{
		{
			name:           "no_filters",
			config:         &MongoDBStreamConfig{},
			expectedStages: 0,
		},
		{
			name: "include_operations",
			config: &MongoDBStreamConfig{
				IncludeOperations: []string{"insert", "update"},
			},
			expectedStages: 1,
		},
		{
			name: "exclude_operations",
			config: &MongoDBStreamConfig{
				ExcludeOperations: []string{"delete"},
			},
			expectedStages: 1,
		},
		{
			name: "both_include_and_exclude",
			config: &MongoDBStreamConfig{
				IncludeOperations: []string{"insert", "update"},
				ExcludeOperations: []string{"delete"},
			},
			expectedStages: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := provider.createPipeline(tt.config)
			assert.Len(t, pipeline, tt.expectedStages)

			if tt.expectedStages > 0 {
				// Verify the pipeline contains a $match stage
				matchStage, exists := pipeline[0]["$match"]
				assert.True(t, exists)
				assert.NotNil(t, matchStage)
			}
		})
	}
}

func TestReadPrefFromString(t *testing.T) {
	tests := []struct {
		name     string
		readPref string
		wantErr  bool
	}{
		{"primary", "primary", false},
		{"secondary", "secondary", false},
		{"primaryPreferred", "primaryPreferred", false},
		{"secondaryPreferred", "secondaryPreferred", false},
		{"nearest", "nearest", false},
		{"invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := readPrefFromString(tt.readPref)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestMongoDBStreamProvider_processChangeEvent(t *testing.T) {
	// Create event channel
	eventChan := make(chan *events.RecordEvent, 100)
	
	// Create a mock MongoDB stream provider
	provider := &MongoDBStreamProvider{
		events: &eventChan,
		config: &config.WaterFlowsConfig{
			Type:   "mongodb",
			Schema: "testdb",
		},
	}

	tests := []struct {
		name           string
		mongoEvent     MongoDBChangeEvent
		expectedAction string
		expectedSchema string
		expectedColl   string
		wantErr        bool
	}{
		{
			name: "insert_operation",
			mongoEvent: MongoDBChangeEvent{
				ID:            bson.M{"_data": "test"},
				OperationType: "insert",
				ClusterTime:   primitive.Timestamp{T: uint32(time.Now().Unix()), I: 0},
				FullDocument: bson.M{
					"_id":  primitive.NewObjectID(),
					"name": "test",
				},
				DocumentKey: bson.M{"_id": primitive.NewObjectID()},
				Namespace: Namespace{
					Database:   "testdb",
					Collection: "testcoll",
				},
			},
			expectedAction: events.InsertAction,
			expectedSchema: "testdb",
			expectedColl:   "testcoll",
			wantErr:        false,
		},
		{
			name: "update_operation",
			mongoEvent: MongoDBChangeEvent{
				ID:            bson.M{"_data": "test"},
				OperationType: "update",
				ClusterTime:   primitive.Timestamp{T: uint32(time.Now().Unix()), I: 0},
				FullDocument: bson.M{
					"_id":  primitive.NewObjectID(),
					"name": "updated_name",
				},
				UpdateDescription: &UpdateDescription{
					UpdatedFields: map[string]interface{}{
						"name": "updated_name",
					},
					RemovedFields: []string{"old_field"},
				},
				DocumentKey: bson.M{"_id": primitive.NewObjectID()},
				Namespace: Namespace{
					Database:   "testdb",
					Collection: "testcoll",
				},
			},
			expectedAction: events.UpdateAction,
			expectedSchema: "testdb",
			expectedColl:   "testcoll",
			wantErr:        false,
		},
		{
			name: "delete_operation",
			mongoEvent: MongoDBChangeEvent{
				ID:            bson.M{"_data": "test"},
				OperationType: "delete",
				ClusterTime:   primitive.Timestamp{T: uint32(time.Now().Unix()), I: 0},
				DocumentKey:   bson.M{"_id": primitive.NewObjectID()},
				Namespace: Namespace{
					Database:   "testdb",
					Collection: "testcoll",
				},
			},
			expectedAction: events.DeleteAction,
			expectedSchema: "testdb",
			expectedColl:   "testcoll",
			wantErr:        false,
		},
		{
			name: "unsupported_operation",
			mongoEvent: MongoDBChangeEvent{
				ID:            bson.M{"_data": "test"},
				OperationType: "invalidate",
				ClusterTime:   primitive.Timestamp{T: uint32(time.Now().Unix()), I: 0},
				Namespace: Namespace{
					Database:   "testdb",
					Collection: "testcoll",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.processChangeEvent(tt.mongoEvent)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)

				// Check that an event was sent to the channel
				select {
				case recordEvent := <-eventChan:
					assert.Equal(t, tt.expectedAction, recordEvent.Action)
					assert.Equal(t, tt.expectedSchema, recordEvent.Schema)
					assert.Equal(t, tt.expectedColl, recordEvent.Collection)
					
					// Verify data is valid JSON if present
					if len(recordEvent.Data) > 0 {
						var data map[string]interface{}
						err := json.Unmarshal(recordEvent.Data, &data)
						assert.NoError(t, err)
					}
					
					// Verify old data is valid JSON if present
					if len(recordEvent.OldData) > 0 {
						var oldData events.RecordKey
						err := json.Unmarshal(recordEvent.OldData, &oldData)
						assert.NoError(t, err)
					}

				case <-time.After(100 * time.Millisecond):
					t.Error("Expected event to be sent to channel")
				}
			}
		})
	}
}

func TestMongoDBStreamProvider_isFatalError(t *testing.T) {
	provider := &MongoDBStreamProvider{}
	
	// Create mock errors for testing
	timeoutErr := &mockError{timeout: true}
	networkErr := &mockError{network: true}
	// Use a simpler approach for command errors - just check the error type behavior
	normalErr := &mockError{}

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"timeout_error", timeoutErr, false},
		{"network_error", networkErr, false},
		{"normal_error", normalErr, false}, // Since our mock doesn't implement CommandError, it should be non-fatal
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.isFatalError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMongoDBStreamProvider_Stop(t *testing.T) {
	provider := &MongoDBStreamProvider{
		isRunning: true,
		stopChan:  make(chan struct{}),
	}

	// Stop should close the channel and set isRunning to false
	provider.Stop()
	
	assert.False(t, provider.isRunning)
	
	// Verify channel is closed
	select {
	case <-provider.stopChan:
		// Channel is closed, which is expected
	default:
		t.Error("Expected stop channel to be closed")
	}
}

func TestMongoDBStreamProvider_StreamType(t *testing.T) {
	provider := &MongoDBStreamProvider{}
	assert.Equal(t, "MongoDB", provider.StreamType())
}

// Mock error types for testing
type mockError struct {
	timeout bool
	network bool
}

func (e *mockError) Error() string {
	return "mock error"
}

func (e *mockError) Timeout() bool {
	return e.timeout
}

func (e *mockError) Network() bool {
	return e.network
}

// Benchmark tests for MongoDB stream provider
func BenchmarkMongoDBStreamProvider_parseMongoDBConfig(b *testing.B) {
	provider := &MongoDBStreamProvider{
		config: &config.WaterFlowsConfig{
			Type:       "mongodb",
			Host:       "localhost",
			Port:       27017,
			Schema:     "testdb",
			Collection: "testcoll",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := provider.parseMongoDBConfig()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMongoDBStreamProvider_processChangeEvent(b *testing.B) {
	eventChan := make(chan *events.RecordEvent, 1000)
	
	provider := &MongoDBStreamProvider{
		events: &eventChan,
		config: &config.WaterFlowsConfig{
			Type:   "mongodb",
			Schema: "testdb",
		},
	}

	mongoEvent := MongoDBChangeEvent{
		ID:            bson.M{"_data": "test"},
		OperationType: "insert",
		ClusterTime:   primitive.Timestamp{T: uint32(time.Now().Unix()), I: 0},
		FullDocument: bson.M{
			"_id":  primitive.NewObjectID(),
			"name": "test",
		},
		DocumentKey: bson.M{"_id": primitive.NewObjectID()},
		Namespace: Namespace{
			Database:   "testdb",
			Collection: "testcoll",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := provider.processChangeEvent(mongoEvent)
		if err != nil {
			b.Fatal(err)
		}
		// Drain the channel to prevent blocking
		<-eventChan
	}
}