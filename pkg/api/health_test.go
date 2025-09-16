package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthResponse(t *testing.T) {
	response := HealthResponse{
		Status:    HealthStatusHealthy,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    "1h30m",
		Checks: map[string]CheckResult{
			"database": {
				Status:      HealthStatusHealthy,
				Message:     "connection successful",
				Duration:    time.Millisecond * 10,
				Timestamp:   time.Now(),
				ComponentID: "db-primary",
			},
		},
		Environment: "production",
	}
	
	assert.Equal(t, HealthStatusHealthy, response.Status)
	assert.Equal(t, "1.0.0", response.Version)
	assert.Equal(t, "1h30m", response.Uptime)
	assert.Equal(t, "production", response.Environment)
	assert.Len(t, response.Checks, 1)
	assert.Equal(t, HealthStatusHealthy, response.Checks["database"].Status)
}

func TestCheckResult(t *testing.T) {
	result := CheckResult{
		Status:      HealthStatusDegraded,
		Message:     "high latency detected",
		Duration:    time.Millisecond * 500,
		Timestamp:   time.Now(),
		ComponentID: "stream-processor",
	}
	
	assert.Equal(t, HealthStatusDegraded, result.Status)
	assert.Equal(t, "high latency detected", result.Message)
	assert.Equal(t, time.Millisecond*500, result.Duration)
	assert.Equal(t, "stream-processor", result.ComponentID)
	assert.Empty(t, result.Error)
}

func TestNewHealthService(t *testing.T) {
	cfg := config.DefaultConfig()
	service := NewHealthService(cfg, nil)
	
	require.NotNil(t, service)
	assert.Equal(t, cfg, service.cfg)
	assert.Equal(t, "1.0.0", service.version)
	assert.Len(t, service.checkers, 0)
	assert.False(t, service.startTime.IsZero())
}

func TestHealthServiceRegisterChecker(t *testing.T) {
	service := NewHealthService(nil, nil)
	
	checker := &mockHealthChecker{
		name:      "test-checker",
		essential: true,
		result: CheckResult{
			Status:  HealthStatusHealthy,
			Message: "test passed",
		},
	}
	
	service.RegisterChecker(checker)
	
	assert.Len(t, service.checkers, 1)
	assert.Equal(t, "test-checker", service.checkers[0].Name())
}

func TestHealthServicePerformHealthCheck(t *testing.T) {
	tests := []struct {
		name           string
		checkers       []HealthChecker
		expectedStatus HealthStatus
	}{
		{
			name: "all healthy",
			checkers: []HealthChecker{
				&mockHealthChecker{
					name:      "check1",
					essential: true,
					result:    CheckResult{Status: HealthStatusHealthy, Message: "ok"},
				},
				&mockHealthChecker{
					name:      "check2",
					essential: false,
					result:    CheckResult{Status: HealthStatusHealthy, Message: "ok"},
				},
			},
			expectedStatus: HealthStatusHealthy,
		},
		{
			name: "non-essential degraded",
			checkers: []HealthChecker{
				&mockHealthChecker{
					name:      "check1",
					essential: true,
					result:    CheckResult{Status: HealthStatusHealthy, Message: "ok"},
				},
				&mockHealthChecker{
					name:      "check2",
					essential: false,
					result:    CheckResult{Status: HealthStatusDegraded, Message: "slow"},
				},
			},
			expectedStatus: HealthStatusDegraded,
		},
		{
			name: "essential unhealthy",
			checkers: []HealthChecker{
				&mockHealthChecker{
					name:      "check1",
					essential: true,
					result:    CheckResult{Status: HealthStatusUnhealthy, Error: "failed"},
				},
				&mockHealthChecker{
					name:      "check2",
					essential: false,
					result:    CheckResult{Status: HealthStatusHealthy, Message: "ok"},
				},
			},
			expectedStatus: HealthStatusUnhealthy,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewHealthService(nil, nil)
			
			for _, checker := range tt.checkers {
				service.RegisterChecker(checker)
			}
			
			response := service.PerformHealthCheck()
			
			assert.Equal(t, tt.expectedStatus, response.Status)
			assert.Equal(t, "1.0.0", response.Version)
			assert.NotEmpty(t, response.Uptime)
			assert.Len(t, response.Checks, len(tt.checkers))
			
			for _, checker := range tt.checkers {
				check, exists := response.Checks[checker.Name()]
				assert.True(t, exists)
				assert.False(t, check.Timestamp.IsZero())
				assert.Greater(t, check.Duration, time.Duration(0))
			}
		})
	}
}

func TestHealthServiceGetEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expected    string
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: "production",
		},
		{
			name: "debug level",
			config: &config.Config{
				Logging: config.LoggingConfig{Level: "debug"},
			},
			expected: "development",
		},
		{
			name: "info level",
			config: &config.Config{
				Logging: config.LoggingConfig{Level: "info"},
			},
			expected: "production",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewHealthService(tt.config, nil)
			result := service.getEnvironment()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewHealthHandler(t *testing.T) {
	service := NewHealthService(nil, nil)
	handler := NewHealthHandler(service)
	
	require.NotNil(t, handler)
	assert.Equal(t, service, handler.healthService)
}

func TestHealthHandlerServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		healthStatus   HealthStatus
		expectedCode   int
		expectJSON     bool
	}{
		{
			name:         "GET healthy",
			method:       http.MethodGet,
			healthStatus: HealthStatusHealthy,
			expectedCode: http.StatusOK,
			expectJSON:   true,
		},
		{
			name:         "GET degraded",
			method:       http.MethodGet,
			healthStatus: HealthStatusDegraded,
			expectedCode: http.StatusOK,
			expectJSON:   true,
		},
		{
			name:         "GET unhealthy",
			method:       http.MethodGet,
			healthStatus: HealthStatusUnhealthy,
			expectedCode: http.StatusServiceUnavailable,
			expectJSON:   true,
		},
		{
			name:         "POST not allowed",
			method:       http.MethodPost,
			healthStatus: HealthStatusHealthy,
			expectedCode: http.StatusMethodNotAllowed,
			expectJSON:   false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create service with mock checker
			service := NewHealthService(nil, nil)
			service.RegisterChecker(&mockHealthChecker{
				name:      "test",
				essential: true,
				result:    CheckResult{Status: tt.healthStatus, Message: "test"},
			})
			
			handler := NewHealthHandler(service)
			
			// Create request
			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()
			
			// Execute request
			handler.ServeHTTP(w, req)
			
			// Verify response
			assert.Equal(t, tt.expectedCode, w.Code)
			
			if tt.expectJSON {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
				assert.Equal(t, "no-cache, no-store, must-revalidate", w.Header().Get("Cache-Control"))
				
				var response HealthResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, tt.healthStatus, response.Status)
				assert.Equal(t, "1.0.0", response.Version)
				assert.NotEmpty(t, response.Uptime)
				assert.Len(t, response.Checks, 1)
			}
		})
	}
}

func TestDatabaseChecker(t *testing.T) {
	tests := []struct {
		name        string
		pingFunc    func() error
		expected    HealthStatus
		expectError bool
	}{
		{
			name:        "successful ping",
			pingFunc:    func() error { return nil },
			expected:    HealthStatusHealthy,
			expectError: false,
		},
		{
			name:        "failed ping",
			pingFunc:    func() error { return errors.New("connection failed") },
			expected:    HealthStatusUnhealthy,
			expectError: true,
		},
		{
			name:        "nil ping function",
			pingFunc:    nil,
			expected:    HealthStatusUnhealthy,
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewDatabaseChecker("test-db", true, tt.pingFunc, "db-1")
			
			assert.Equal(t, "test-db", checker.Name())
			assert.True(t, checker.IsEssential())
			
			result := checker.Check()
			assert.Equal(t, tt.expected, result.Status)
			assert.Equal(t, "db-1", result.ComponentID)
			
			if tt.expectError {
				assert.NotEmpty(t, result.Error)
			} else {
				assert.Empty(t, result.Error)
				assert.NotEmpty(t, result.Message)
			}
		})
	}
}

func TestStreamChecker(t *testing.T) {
	tests := []struct {
		name           string
		getStreamCount func() (active, total int, err error)
		expected       HealthStatus
		expectError    bool
	}{
		{
			name:           "all streams active",
			getStreamCount: func() (int, int, error) { return 5, 5, nil },
			expected:       HealthStatusHealthy,
			expectError:    false,
		},
		{
			name:           "some streams inactive",
			getStreamCount: func() (int, int, error) { return 3, 5, nil },
			expected:       HealthStatusDegraded,
			expectError:    false,
		},
		{
			name:           "no streams active",
			getStreamCount: func() (int, int, error) { return 0, 5, nil },
			expected:       HealthStatusUnhealthy,
			expectError:    false,
		},
		{
			name:           "no streams configured",
			getStreamCount: func() (int, int, error) { return 0, 0, nil },
			expected:       HealthStatusHealthy,
			expectError:    false,
		},
		{
			name:           "error getting stream count",
			getStreamCount: func() (int, int, error) { return 0, 0, errors.New("failed") },
			expected:       HealthStatusUnhealthy,
			expectError:    true,
		},
		{
			name:           "nil function",
			getStreamCount: nil,
			expected:       HealthStatusUnhealthy,
			expectError:    true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewStreamChecker("stream-health", false, tt.getStreamCount, "stream-1")
			
			assert.Equal(t, "stream-health", checker.Name())
			assert.False(t, checker.IsEssential())
			
			result := checker.Check()
			assert.Equal(t, tt.expected, result.Status)
			assert.Equal(t, "stream-1", result.ComponentID)
			
			if tt.expectError {
				assert.NotEmpty(t, result.Error)
			} else {
				assert.NotEmpty(t, result.Message)
			}
		})
	}
}

func TestMemoryChecker(t *testing.T) {
	tests := []struct {
		name        string
		maxMemoryMB int64
		getMemoryMB func() (int64, error)
		expected    HealthStatus
		expectError bool
	}{
		{
			name:        "normal memory usage",
			maxMemoryMB: 1000,
			getMemoryMB: func() (int64, error) { return 500, nil },
			expected:    HealthStatusHealthy,
			expectError: false,
		},
		{
			name:        "high memory usage",
			maxMemoryMB: 1000,
			getMemoryMB: func() (int64, error) { return 850, nil }, // 85% of 1000
			expected:    HealthStatusDegraded,
			expectError: false,
		},
		{
			name:        "memory limit exceeded",
			maxMemoryMB: 1000,
			getMemoryMB: func() (int64, error) { return 1200, nil },
			expected:    HealthStatusUnhealthy,
			expectError: false,
		},
		{
			name:        "error getting memory",
			maxMemoryMB: 1000,
			getMemoryMB: func() (int64, error) { return 0, errors.New("failed") },
			expected:    HealthStatusUnhealthy,
			expectError: true,
		},
		{
			name:        "nil function",
			maxMemoryMB: 1000,
			getMemoryMB: nil,
			expected:    HealthStatusUnhealthy,
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewMemoryChecker("memory", false, tt.maxMemoryMB, tt.getMemoryMB, "memory-1")
			
			assert.Equal(t, "memory", checker.Name())
			assert.False(t, checker.IsEssential())
			
			result := checker.Check()
			assert.Equal(t, tt.expected, result.Status)
			assert.Equal(t, "memory-1", result.ComponentID)
			
			if tt.expectError {
				assert.NotEmpty(t, result.Error)
			} else {
				assert.NotEmpty(t, result.Message)
			}
		})
	}
}

// Mock health checker for testing
type mockHealthChecker struct {
	name      string
	essential bool
	result    CheckResult
}

func (m *mockHealthChecker) Name() string {
	return m.name
}

func (m *mockHealthChecker) IsEssential() bool {
	return m.essential
}

func (m *mockHealthChecker) Check() CheckResult {
	return m.result
}