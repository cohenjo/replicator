package position

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseTracker_MongoDB(t *testing.T) {
	config := &DatabaseConfig{
		Type:              "mongodb",
		ConnectionString:  "mongodb://localhost:27017",
		Schema:           "test_db",
		CollectionName:   "test_positions",
		UseTransactions:  true,
		EnableAutoMigration: true,
	}
	
	tracker, err := NewDatabaseTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer tracker.Close()
	
	assert.NotNil(t, tracker)
	assert.Equal(t, "mongodb", tracker.GetDatabaseType())
	assert.IsType(t, &MongoTracker{}, tracker.GetUnderlyingTracker())
}

func TestDatabaseTracker_MongoWithConfig(t *testing.T) {
	mongoConfig := &MongoConfig{
		ConnectionURI: "mongodb://localhost:27017",
		Database:      "test_db",
		Collection:    "custom_positions",
	}
	
	config := &DatabaseConfig{
		Type:        "mongodb",
		MongoConfig: mongoConfig,
	}
	
	tracker, err := NewDatabaseTracker(config)
	if err != nil {
		t.Skip("MongoDB not available, skipping integration test")
		return
	}
	defer tracker.Close()
	
	assert.NotNil(t, tracker)
	assert.Equal(t, "mongodb", tracker.GetDatabaseType())
	
	// Verify the underlying tracker is properly configured
	mongoTracker, ok := tracker.GetUnderlyingTracker().(*MongoTracker)
	require.True(t, ok)
	assert.Equal(t, "custom_positions", mongoTracker.config.Collection)
}

func TestDatabaseTracker_UnsupportedType(t *testing.T) {
	config := &DatabaseConfig{
		Type:             "unsupported",
		ConnectionString: "some://connection",
	}
	
	tracker, err := NewDatabaseTracker(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database type")
	assert.Nil(t, tracker)
}

func TestDatabaseTracker_MissingConfig(t *testing.T) {
	tracker, err := NewDatabaseTracker(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database config is required")
	assert.Nil(t, tracker)
}

func TestDatabaseTracker_MissingType(t *testing.T) {
	config := &DatabaseConfig{
		ConnectionString: "mongodb://localhost:27017",
	}
	
	tracker, err := NewDatabaseTracker(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database type is required")
	assert.Nil(t, tracker)
}

func TestDatabaseTracker_MySQLNotImplemented(t *testing.T) {
	config := &DatabaseConfig{
		Type:             "mysql",
		ConnectionString: "user:pass@tcp(localhost:3306)/db",
	}
	
	tracker, err := NewDatabaseTracker(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MySQL database tracker not implemented yet")
	assert.Nil(t, tracker)
}

func TestDatabaseTracker_PostgreSQLNotImplemented(t *testing.T) {
	config := &DatabaseConfig{
		Type:             "postgres",
		ConnectionString: "postgres://user:pass@localhost/db",
	}
	
	tracker, err := NewDatabaseTracker(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PostgreSQL database tracker not implemented yet")
	assert.Nil(t, tracker)
}