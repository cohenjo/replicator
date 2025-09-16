
package streams

import (
"testing"
"github.com/cohenjo/replicator/pkg/config"
"github.com/cohenjo/replicator/pkg/events"
"github.com/stretchr/testify/assert"
)

func TestNewMongoDBStream(t *testing.T) {
// Create test configuration
streamConfig := config.StreamConfig{
Name: "test-stream",
Source: config.SourceConfig{
Type:     "mongodb",
URI:      "mongodb://admin:password123@localhost:27017/test_db?authSource=admin",
Database: "test_db",
Options: map[string]interface{}{
"collection": "test_collection",
},
},
}

// Create event channel
eventChannel := make(chan events.RecordEvent, 10)

// Test stream creation
stream, err := NewMongoDBStream(streamConfig, eventChannel)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	assert.Equal(t, "test-stream", stream.GetConfig().Name)
	assert.Equal(t, config.SourceType("mongodb"), stream.GetConfig().Source.Type)// Test state
state := stream.GetState()
assert.Equal(t, "test-stream", state.Name)
assert.Equal(t, config.StreamStatusStopped, state.Status)

// Test metrics
metrics := stream.GetMetrics()
assert.Equal(t, "test-stream", metrics.StreamName)
assert.Equal(t, int64(0), metrics.EventsProcessed)
}

func TestMongoDBStreamInvalidConfig(t *testing.T) {
// Create test configuration with invalid type
streamConfig := config.StreamConfig{
Name: "test-stream",
Source: config.SourceConfig{
Type: "mysql", // Invalid for MongoDB stream
},
}

// Create event channel
eventChannel := make(chan events.RecordEvent, 10)

// Test stream creation should fail
stream, err := NewMongoDBStream(streamConfig, eventChannel)

// Assertions
assert.Error(t, err)
assert.Nil(t, stream)
assert.Contains(t, err.Error(), "invalid source type")
}

