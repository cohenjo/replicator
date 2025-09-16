package estuary

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDestinationRecord(t *testing.T) {
	record := DestinationRecord{
		Table:     "users",
		Operation: "INSERT",
		Data: map[string]interface{}{
			"id":   1,
			"name": "John Doe",
		},
		Key: map[string]interface{}{
			"id": 1,
		},
		Timestamp: time.Now(),
	}
	
	assert.Equal(t, "users", record.Table)
	assert.Equal(t, "INSERT", record.Operation)
	assert.Equal(t, 1, record.Data["id"])
	assert.Equal(t, "John Doe", record.Data["name"])
	assert.Equal(t, 1, record.Key["id"])
	assert.False(t, record.Timestamp.IsZero())
}

func TestTableSchema(t *testing.T) {
	schema := TableSchema{
		Name: "users",
		Columns: []ColumnDefinition{
			{
				Name:     "id",
				Type:     "INTEGER",
				Nullable: false,
			},
			{
				Name:     "name",
				Type:     "VARCHAR",
				Nullable: true,
				MaxLength: &[]int{255}[0],
			},
		},
		PrimaryKey: []string{"id"},
		Indexes: []IndexDefinition{
			{
				Name:    "idx_name",
				Columns: []string{"name"},
				Unique:  false,
			},
		},
	}
	
	assert.Equal(t, "users", schema.Name)
	assert.Len(t, schema.Columns, 2)
	assert.Equal(t, "id", schema.Columns[0].Name)
	assert.Equal(t, "INTEGER", schema.Columns[0].Type)
	assert.False(t, schema.Columns[0].Nullable)
	assert.Equal(t, "name", schema.Columns[1].Name)
	assert.Equal(t, "VARCHAR", schema.Columns[1].Type)
	assert.True(t, schema.Columns[1].Nullable)
	assert.NotNil(t, schema.Columns[1].MaxLength)
	assert.Equal(t, 255, *schema.Columns[1].MaxLength)
	assert.Equal(t, []string{"id"}, schema.PrimaryKey)
	assert.Len(t, schema.Indexes, 1)
	assert.Equal(t, "idx_name", schema.Indexes[0].Name)
}

func TestColumnDefinition(t *testing.T) {
	tests := []struct {
		name   string
		column ColumnDefinition
	}{
		{
			name: "integer column",
			column: ColumnDefinition{
				Name:         "id",
				Type:         "INTEGER",
				Nullable:     false,
				DefaultValue: 0,
			},
		},
		{
			name: "varchar column with length",
			column: ColumnDefinition{
				Name:      "name",
				Type:      "VARCHAR",
				Nullable:  true,
				MaxLength: &[]int{100}[0],
			},
		},
		{
			name: "decimal column with precision and scale",
			column: ColumnDefinition{
				Name:      "price",
				Type:      "DECIMAL",
				Nullable:  false,
				Precision: &[]int{10}[0],
				Scale:     &[]int{2}[0],
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column := tt.column
			assert.NotEmpty(t, column.Name)
			assert.NotEmpty(t, column.Type)
		})
	}
}

func TestIndexDefinition(t *testing.T) {
	index := IndexDefinition{
		Name:    "idx_email",
		Columns: []string{"email"},
		Unique:  true,
		Type:    "BTREE",
	}
	
	assert.Equal(t, "idx_email", index.Name)
	assert.Equal(t, []string{"email"}, index.Columns)
	assert.True(t, index.Unique)
	assert.Equal(t, "BTREE", index.Type)
}

func TestConstraintDefinition(t *testing.T) {
	constraint := ConstraintDefinition{
		Name:    "chk_age",
		Type:    "CHECK",
		Columns: []string{"age"},
		Expression: "age >= 0",
	}
	
	assert.Equal(t, "chk_age", constraint.Name)
	assert.Equal(t, "CHECK", constraint.Type)
	assert.Equal(t, []string{"age"}, constraint.Columns)
	assert.Equal(t, "age >= 0", constraint.Expression)
}

func TestHealthStatus(t *testing.T) {
	now := time.Now()
	health := HealthStatus{
		Status:       "HEALTHY",
		Message:      "All systems operational",
		LastCheck:    now,
		ResponseTime: time.Millisecond * 50,
		Details: map[string]interface{}{
			"connection_pool": "active",
			"last_write":      now.Add(-time.Minute),
		},
	}
	
	assert.Equal(t, "HEALTHY", health.Status)
	assert.Equal(t, "All systems operational", health.Message)
	assert.Equal(t, now, health.LastCheck)
	assert.Equal(t, time.Millisecond*50, health.ResponseTime)
	assert.Equal(t, "active", health.Details["connection_pool"])
}

func TestDestinationMetrics(t *testing.T) {
	metrics := DestinationMetrics{
		TotalConnections:      100,
		ActiveConnections:     5,
		ConnectionErrors:      2,
		AverageConnectTime:    time.Millisecond * 10,
		TotalWrites:          1000,
		SuccessfulWrites:     995,
		FailedWrites:         5,
		AverageWriteTime:     time.Millisecond * 5,
		BytesWritten:         1024000,
		RecordsWritten:       1000,
		TotalTransactions:    50,
		CommittedTransactions: 48,
		RolledBackTransactions: 2,
		AverageTransactionTime: time.Millisecond * 100,
		TotalErrors:          7,
		LastError:            "connection timeout",
		Throughput:           100.5,
		Latency:              time.Millisecond * 2,
		LastUpdated:          time.Now(),
	}
	
	assert.Equal(t, int64(100), metrics.TotalConnections)
	assert.Equal(t, int64(5), metrics.ActiveConnections)
	assert.Equal(t, int64(2), metrics.ConnectionErrors)
	assert.Equal(t, time.Millisecond*10, metrics.AverageConnectTime)
	assert.Equal(t, int64(1000), metrics.TotalWrites)
	assert.Equal(t, int64(995), metrics.SuccessfulWrites)
	assert.Equal(t, int64(5), metrics.FailedWrites)
	assert.Equal(t, time.Millisecond*5, metrics.AverageWriteTime)
	assert.Equal(t, int64(1024000), metrics.BytesWritten)
	assert.Equal(t, int64(1000), metrics.RecordsWritten)
	assert.Equal(t, int64(50), metrics.TotalTransactions)
	assert.Equal(t, int64(48), metrics.CommittedTransactions)
	assert.Equal(t, int64(2), metrics.RolledBackTransactions)
	assert.Equal(t, time.Millisecond*100, metrics.AverageTransactionTime)
	assert.Equal(t, int64(7), metrics.TotalErrors)
	assert.Equal(t, "connection timeout", metrics.LastError)
	assert.Equal(t, 100.5, metrics.Throughput)
	assert.Equal(t, time.Millisecond*2, metrics.Latency)
	assert.False(t, metrics.LastUpdated.IsZero())
}

func TestConnectionPoolConfig(t *testing.T) {
	config := ConnectionPoolConfig{
		MaxConnections:    10,
		MinConnections:    2,
		MaxIdleTime:       time.Minute * 30,
		MaxLifetime:       time.Hour,
		ConnectionTimeout: time.Second * 30,
		ValidationQuery:   "SELECT 1",
	}
	
	assert.Equal(t, 10, config.MaxConnections)
	assert.Equal(t, 2, config.MinConnections)
	assert.Equal(t, time.Minute*30, config.MaxIdleTime)
	assert.Equal(t, time.Hour, config.MaxLifetime)
	assert.Equal(t, time.Second*30, config.ConnectionTimeout)
	assert.Equal(t, "SELECT 1", config.ValidationQuery)
}

func TestRetryConfig(t *testing.T) {
	config := RetryConfig{
		MaxRetries:    3,
		InitialDelay:  time.Millisecond * 100,
		MaxDelay:      time.Second * 10,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"connection timeout",
			"temporary failure",
		},
	}
	
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, time.Millisecond*100, config.InitialDelay)
	assert.Equal(t, time.Second*10, config.MaxDelay)
	assert.Equal(t, 2.0, config.BackoffFactor)
	assert.Len(t, config.RetryableErrors, 2)
	assert.Contains(t, config.RetryableErrors, "connection timeout")
	assert.Contains(t, config.RetryableErrors, "temporary failure")
}

func TestBatchConfig(t *testing.T) {
	config := BatchConfig{
		BatchSize:      100,
		BatchTimeout:   time.Second * 5,
		MaxBatchSize:   1000,
		FlushInterval:  time.Second * 10,
		EnableBatching: true,
	}
	
	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, time.Second*5, config.BatchTimeout)
	assert.Equal(t, 1000, config.MaxBatchSize)
	assert.Equal(t, time.Second*10, config.FlushInterval)
	assert.True(t, config.EnableBatching)
}

func TestSchemaComparison(t *testing.T) {
	comparison := SchemaComparison{
		TableName: "users",
		AddedColumns: []ColumnDefinition{
			{Name: "email", Type: "VARCHAR", Nullable: true},
		},
		RemovedColumns: []ColumnDefinition{
			{Name: "old_field", Type: "TEXT", Nullable: true},
		},
		ModifiedColumns: []ColumnModification{
			{
				Name: "name",
				OldColumn: ColumnDefinition{
					Name: "name", Type: "VARCHAR", MaxLength: &[]int{100}[0],
				},
				NewColumn: ColumnDefinition{
					Name: "name", Type: "VARCHAR", MaxLength: &[]int{255}[0],
				},
				ChangeType: "LENGTH_CHANGE",
			},
		},
		HasChanges: true,
	}
	
	assert.Equal(t, "users", comparison.TableName)
	assert.Len(t, comparison.AddedColumns, 1)
	assert.Equal(t, "email", comparison.AddedColumns[0].Name)
	assert.Len(t, comparison.RemovedColumns, 1)
	assert.Equal(t, "old_field", comparison.RemovedColumns[0].Name)
	assert.Len(t, comparison.ModifiedColumns, 1)
	assert.Equal(t, "name", comparison.ModifiedColumns[0].Name)
	assert.Equal(t, "LENGTH_CHANGE", comparison.ModifiedColumns[0].ChangeType)
	assert.True(t, comparison.HasChanges)
}

func TestSchemaMigration(t *testing.T) {
	migration := SchemaMigration{
		ID:        "migration_001",
		TableName: "users",
		Operations: []MigrationOperation{
			{
				Type: "ADD_COLUMN",
				Parameters: map[string]interface{}{
					"column_name": "email",
					"column_type": "VARCHAR(255)",
					"nullable":    true,
				},
				SQL:         "ALTER TABLE users ADD COLUMN email VARCHAR(255)",
				Description: "Add email column",
			},
		},
		CreatedAt:   time.Now(),
		Description: "Add email column to users table",
	}
	
	assert.Equal(t, "migration_001", migration.ID)
	assert.Equal(t, "users", migration.TableName)
	assert.Len(t, migration.Operations, 1)
	assert.Equal(t, "ADD_COLUMN", migration.Operations[0].Type)
	assert.Equal(t, "email", migration.Operations[0].Parameters["column_name"])
	assert.Equal(t, "VARCHAR(255)", migration.Operations[0].Parameters["column_type"])
	assert.Equal(t, true, migration.Operations[0].Parameters["nullable"])
	assert.Contains(t, migration.Operations[0].SQL, "ALTER TABLE users ADD COLUMN email")
	assert.Equal(t, "Add email column", migration.Operations[0].Description)
	assert.False(t, migration.CreatedAt.IsZero())
	assert.Equal(t, "Add email column to users table", migration.Description)
}

func TestDestinationError(t *testing.T) {
	cause := errors.New("database connection failed")
	
	err := NewConnectionError("postgres-dest", "failed to connect to database", cause)
	
	assert.Equal(t, ErrCodeConnectionFailed, err.Code)
	assert.Equal(t, "failed to connect to database", err.Message)
	assert.Equal(t, "connect", err.Operation)
	assert.Equal(t, "postgres-dest", err.Destination)
	assert.Equal(t, cause, err.Cause)
	assert.False(t, err.Timestamp.IsZero())
	
	// Test Error() method
	errMsg := err.Error()
	assert.Contains(t, errMsg, ErrCodeConnectionFailed)
	assert.Contains(t, errMsg, "failed to connect to database")
	assert.Contains(t, errMsg, "connect")
	assert.Contains(t, errMsg, "postgres-dest")
	assert.Contains(t, errMsg, "database connection failed")
	
	// Test Unwrap() method
	assert.Equal(t, cause, err.Unwrap())
}

func TestWriteError(t *testing.T) {
	cause := errors.New("table not found")
	
	err := NewWriteError("mysql-dest", "users", "table does not exist", cause)
	
	assert.Equal(t, ErrCodeWriteFailed, err.Code)
	assert.Equal(t, "table does not exist", err.Message)
	assert.Equal(t, "write", err.Operation)
	assert.Equal(t, "mysql-dest", err.Destination)
	assert.Equal(t, "users", err.Details["table"])
	assert.Equal(t, cause, err.Cause)
}

func TestSchemaError(t *testing.T) {
	cause := errors.New("schema validation failed")
	
	err := NewSchemaError("postgres-dest", "orders", "invalid schema format", cause)
	
	assert.Equal(t, ErrCodeSchemaNotFound, err.Code)
	assert.Equal(t, "invalid schema format", err.Message)
	assert.Equal(t, "schema", err.Operation)
	assert.Equal(t, "postgres-dest", err.Destination)
	assert.Equal(t, "orders", err.Details["table"])
	assert.Equal(t, cause, err.Cause)
}

func TestTransactionError(t *testing.T) {
	cause := errors.New("deadlock detected")
	
	err := NewTransactionError("postgres-dest", "transaction failed due to deadlock", cause)
	
	assert.Equal(t, ErrCodeTransactionFailed, err.Code)
	assert.Equal(t, "transaction failed due to deadlock", err.Message)
	assert.Equal(t, "transaction", err.Operation)
	assert.Equal(t, "postgres-dest", err.Destination)
	assert.Equal(t, cause, err.Cause)
}

func TestDestinationErrorWithoutCause(t *testing.T) {
	err := &DestinationError{
		Code:        ErrCodeInvalidConfig,
		Message:     "invalid configuration provided",
		Operation:   "validate",
		Destination: "test-dest",
		Timestamp:   time.Now(),
	}
	
	errMsg := err.Error()
	assert.Contains(t, errMsg, ErrCodeInvalidConfig)
	assert.Contains(t, errMsg, "invalid configuration provided")
	assert.Contains(t, errMsg, "validate")
	assert.Contains(t, errMsg, "test-dest")
	assert.NotContains(t, errMsg, ":")
	
	// Test Unwrap() with no cause
	assert.Nil(t, err.Unwrap())
}