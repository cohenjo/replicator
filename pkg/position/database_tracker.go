package position

import (
	"context"
	"fmt"
)

// DatabaseTracker is a factory for database-based position tracking
type DatabaseTracker struct {
	config     *DatabaseConfig
	underlying Tracker
}

// NewDatabaseTracker creates a new database-based position tracker
func NewDatabaseTracker(config *DatabaseConfig) (*DatabaseTracker, error) {
	if config == nil {
		return nil, fmt.Errorf("database config is required")
	}
	
	if config.Type == "" {
		return nil, fmt.Errorf("database type is required")
	}
	
	var underlying Tracker
	var err error
	
	switch config.Type {
	case "mongodb", "mongo":
		if config.MongoConfig == nil {
			// Create default MongoConfig from DatabaseConfig
			mongoConfig := &MongoConfig{
				ConnectionURI:           config.ConnectionString,
				Database:                config.Schema,
				Collection:              config.CollectionName,
				EnableTransactions:      config.UseTransactions,
				EnableAutoIndexCreation: config.EnableAutoMigration,
				ConnectTimeout:          config.ConnectionTimeout,
				MaxPoolSize:             uint64(config.ConnectionPoolSize),
			}
			
			if mongoConfig.Database == "" && config.Schema != "" {
				mongoConfig.Database = config.Schema
			}
			
			if mongoConfig.Collection == "" {
				mongoConfig.Collection = "stream_positions"
			}
			
			underlying, err = NewMongoTracker(mongoConfig)
		} else {
			underlying, err = NewMongoTracker(config.MongoConfig)
		}
	case "mysql":
		return nil, fmt.Errorf("MySQL database tracker not implemented yet")
	case "postgres", "postgresql":
		return nil, fmt.Errorf("PostgreSQL database tracker not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to create %s tracker: %w", config.Type, err)
	}
	
	return &DatabaseTracker{
		config:     config,
		underlying: underlying,
	}, nil
}

// Save delegates to the underlying tracker
func (dt *DatabaseTracker) Save(ctx context.Context, streamID string, position Position, metadata map[string]interface{}) error {
	if dt.underlying == nil {
		return fmt.Errorf("database tracker not properly initialized")
	}
	return dt.underlying.Save(ctx, streamID, position, metadata)
}

// Load delegates to the underlying tracker
func (dt *DatabaseTracker) Load(ctx context.Context, streamID string) (Position, map[string]interface{}, error) {
	if dt.underlying == nil {
		return nil, nil, fmt.Errorf("database tracker not properly initialized")
	}
	return dt.underlying.Load(ctx, streamID)
}

// Delete delegates to the underlying tracker
func (dt *DatabaseTracker) Delete(ctx context.Context, streamID string) error {
	if dt.underlying == nil {
		return fmt.Errorf("database tracker not properly initialized")
	}
	return dt.underlying.Delete(ctx, streamID)
}

// List delegates to the underlying tracker
func (dt *DatabaseTracker) List(ctx context.Context) (map[string]Position, error) {
	if dt.underlying == nil {
		return nil, fmt.Errorf("database tracker not properly initialized")
	}
	return dt.underlying.List(ctx)
}

// Close delegates to the underlying tracker
func (dt *DatabaseTracker) Close() error {
	if dt.underlying == nil {
		return nil
	}
	return dt.underlying.Close()
}

// HealthCheck delegates to the underlying tracker
func (dt *DatabaseTracker) HealthCheck(ctx context.Context) error {
	if dt.underlying == nil {
		return fmt.Errorf("database tracker not properly initialized")
	}
	return dt.underlying.HealthCheck(ctx)
}

// GetDatabaseType returns the configured database type
func (dt *DatabaseTracker) GetDatabaseType() string {
	return dt.config.Type
}

// GetUnderlyingTracker returns the underlying tracker for advanced operations
func (dt *DatabaseTracker) GetUnderlyingTracker() Tracker {
	return dt.underlying
}