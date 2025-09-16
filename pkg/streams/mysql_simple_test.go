package streams

import (
	"testing"

	"github.com/cohenjo/replicator/pkg/events"
	"github.com/cohenjo/replicator/pkg/position"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMySQLStreamProvider tests the creation of a new MySQL stream provider
func TestNewMySQLStreamProvider(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	
	provider := NewMySQLStreamProvider(eventSender, logger)
	
	assert.NotNil(t, provider)
	assert.Equal(t, "mysql", provider.StreamType())
	assert.NotNil(t, provider.eventSender)
	assert.NotNil(t, provider.logger)
	assert.NotNil(t, provider.stopChannel)
	assert.NotNil(t, provider.tableSchemas)
}

// TestMySQLStreamProvider_Filtering tests the filtering functionality
func TestMySQLStreamProvider_Filtering(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	
	provider := NewMySQLStreamProvider(eventSender, logger)
	
	// Set up filter configuration
	provider.config = &MySQLConfig{
		IncludeTables:     []string{"users", "orders"},
		ExcludeTables:     []string{"logs"},
		IncludeOperations: []string{"insert", "update"},
		ExcludeOperations: []string{"delete"},
	}
	
	// Test table filtering
	assert.False(t, provider.shouldFilterTable("test", "users"))    // included
	assert.False(t, provider.shouldFilterTable("test", "orders"))   // included
	assert.True(t, provider.shouldFilterTable("test", "logs"))      // excluded
	assert.True(t, provider.shouldFilterTable("test", "products"))  // not included
	
	// Test operation filtering
	assert.False(t, provider.shouldFilterOperation("insert"))  // included
	assert.False(t, provider.shouldFilterOperation("update"))  // included
	assert.True(t, provider.shouldFilterOperation("delete"))   // excluded
	assert.True(t, provider.shouldFilterOperation("truncate")) // not included
}

// TestMySQLStreamProvider_ErrorClassification tests error classification
func TestMySQLStreamProvider_ErrorClassification(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	
	provider := NewMySQLStreamProvider(eventSender, logger)
	
	// Test fatal errors
	fatalErrors := []error{
		assert.AnError, // Mock error for testing
	}
	
	for _, err := range fatalErrors {
		// Note: isFatalError is based on error message content
		// For actual testing, we'd need real MySQL errors
		isFatal := provider.isFatalError(err)
		// Since our mock error doesn't match known patterns, it should be retryable
		assert.False(t, isFatal)
	}
	
	// Test nil error
	assert.False(t, provider.isFatalError(nil))
}

// TestMySQLStreamProvider_PositionTracking tests position tracking functionality
func TestMySQLStreamProvider_PositionTracking(t *testing.T) {
	eventSender := make(chan events.RecordEvent, 10)
	logger := logrus.New()
	
	provider := NewMySQLStreamProvider(eventSender, logger)
	
	// Setup a basic configuration
	provider.config = &MySQLConfig{
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
		Username: "test",
		Password: "test",
		PositionTracking: &position.Config{
			Type: "file",
			FileConfig: &position.FileConfig{
				Directory: t.TempDir(),
			},
		},
	}
	
	// Test position tracking setup
	err := provider.setupPositionTracking()
	require.NoError(t, err)
	assert.NotNil(t, provider.positionTracker)
	assert.NotEmpty(t, provider.streamID)
	
	// Test position saving (should not error even without a real position)
	err = provider.saveCurrentPosition()
	assert.NoError(t, err)
	
	// Cleanup
	provider.cleanupPositionTracking()
}