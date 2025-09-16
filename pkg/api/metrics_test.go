package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cohenjo/replicator/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricsService(t *testing.T) {
	// Test creating a new metrics service
	telemetryManager := &metrics.TelemetryManager{}
	service := NewMetricsService(telemetryManager)

	assert.NotNil(t, service)
	assert.Equal(t, telemetryManager, service.telemetry)
	assert.Equal(t, "1.0.0", service.version)
	assert.Equal(t, "healthy", service.healthStatus)
	assert.Equal(t, int64(0), service.requestCount)
	assert.Equal(t, int64(0), service.errorCount)
	assert.WithinDuration(t, time.Now(), service.startTime, time.Second)
}

func TestMetricsService_GetServiceMetrics(t *testing.T) {
	service := NewMetricsService(nil)
	service.requestCount = 100
	service.errorCount = 5
	service.lastRequest = time.Now().Add(-time.Minute)

	metrics := service.GetServiceMetrics()

	assert.Equal(t, "1.0.0", metrics.Version)
	assert.Equal(t, int64(100), metrics.RequestCount)
	assert.Equal(t, int64(5), metrics.ErrorCount)
	assert.Equal(t, "healthy", metrics.HealthStatus)
	assert.Contains(t, metrics.Uptime, "s") // Should contain seconds
	assert.False(t, metrics.LastRequestTime.IsZero())
}

func TestMetricsService_GetSystemMetrics(t *testing.T) {
	service := NewMetricsService(nil)

	metrics := service.GetSystemMetrics()

	// These are placeholder values until actual implementation
	assert.Equal(t, int64(100), metrics.MemoryUsageMB)
	assert.Equal(t, 15.5, metrics.CPUUsagePercent)
	assert.Equal(t, 50, metrics.GoroutineCount)
	assert.Equal(t, 0.5, metrics.GCPauseMs)
	assert.Equal(t, int64(10), metrics.OpenConnections)
}

func TestMetricsService_GetStreamMetrics(t *testing.T) {
	service := NewMetricsService(nil)

	metrics := service.GetStreamMetrics()

	// Verify expected metrics exist
	assert.Contains(t, metrics, "total_streams")
	assert.Contains(t, metrics, "active_streams")
	assert.Contains(t, metrics, "paused_streams")
	assert.Contains(t, metrics, "failed_streams")
	assert.Contains(t, metrics, "events_processed")
	assert.Contains(t, metrics, "events_per_second")
	assert.Contains(t, metrics, "total_lag_ms")

	// Verify values are correct type
	assert.IsType(t, 0, metrics["total_streams"])
	assert.IsType(t, 0, metrics["active_streams"])
	assert.IsType(t, 0.0, metrics["events_per_second"])
}

func TestMetricsService_RecordRequest(t *testing.T) {
	service := NewMetricsService(nil)
	initialCount := service.requestCount
	initialErrors := service.errorCount

	// Record successful request
	service.RecordRequest("GET", "/test", time.Millisecond*100, 200)
	assert.Equal(t, initialCount+1, service.requestCount)
	assert.Equal(t, initialErrors, service.errorCount)
	assert.WithinDuration(t, time.Now(), service.lastRequest, time.Second)

	// Record error request
	service.RecordRequest("POST", "/test", time.Millisecond*200, 500)
	assert.Equal(t, initialCount+2, service.requestCount)
	assert.Equal(t, initialErrors+1, service.errorCount)
}

func TestMetricsService_SetHealthStatus(t *testing.T) {
	service := NewMetricsService(nil)
	
	service.SetHealthStatus("unhealthy")
	assert.Equal(t, "unhealthy", service.healthStatus)
	
	metrics := service.GetServiceMetrics()
	assert.Equal(t, "unhealthy", metrics.HealthStatus)
}

func TestMetricsService_GetMetrics(t *testing.T) {
	service := NewMetricsService(nil)
	service.requestCount = 50
	service.errorCount = 2

	metricsResponse := service.GetMetrics()

	assert.WithinDuration(t, time.Now(), metricsResponse.Timestamp, time.Second)
	assert.Equal(t, int64(50), metricsResponse.Service.RequestCount)
	assert.Equal(t, int64(2), metricsResponse.Service.ErrorCount)
	assert.NotEmpty(t, metricsResponse.Streams)
	assert.NotZero(t, metricsResponse.System.MemoryUsageMB)
}

func TestNewMetricsHandler(t *testing.T) {
	service := NewMetricsService(nil)
	handler := NewMetricsHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.metricsService)
}

func TestMetricsHandler_ServeHTTP_JSONFormat(t *testing.T) {
	service := NewMetricsService(nil)
	handler := NewMetricsHandler(service)

	// Test successful JSON request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))

	// Verify JSON response
	var response MetricsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotZero(t, response.Timestamp)
	assert.Equal(t, "1.0.0", response.Service.Version)
}

func TestMetricsHandler_ServeHTTP_JSONFormatExplicit(t *testing.T) {
	service := NewMetricsService(nil)
	handler := NewMetricsHandler(service)

	// Test explicit JSON format request
	req := httptest.NewRequest("GET", "/metrics?format=json", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestMetricsHandler_ServeHTTP_PrometheusFormat(t *testing.T) {
	service := NewMetricsService(nil)
	service.requestCount = 100
	service.errorCount = 5
	handler := NewMetricsHandler(service)

	// Test Prometheus format request
	req := httptest.NewRequest("GET", "/metrics?format=prometheus", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain; version=0.0.4; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))

	// Verify Prometheus format content
	body := w.Body.String()
	assert.Contains(t, body, "replicator_requests_total 100")
	assert.Contains(t, body, "replicator_errors_total 5")
	assert.Contains(t, body, "replicator_memory_usage_bytes")
	assert.Contains(t, body, "replicator_cpu_usage_percent")
	assert.Contains(t, body, "replicator_goroutines_count")
	assert.Contains(t, body, "replicator_streams_total")
	assert.Contains(t, body, "replicator_active_streams")
	assert.Contains(t, body, "replicator_events_processed_total")
	assert.Contains(t, body, "replicator_events_per_second")
}

func TestMetricsHandler_ServeHTTP_InvalidMethod(t *testing.T) {
	service := NewMetricsService(nil)
	handler := NewMetricsHandler(service)

	// Test invalid method
	req := httptest.NewRequest("POST", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Contains(t, w.Body.String(), "Method not allowed")
}

func TestMetricsHandler_ServeHTTP_UnsupportedFormat(t *testing.T) {
	service := NewMetricsService(nil)
	handler := NewMetricsHandler(service)

	// Test unsupported format
	req := httptest.NewRequest("GET", "/metrics?format=xml", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Unsupported format")
}

func TestMetricsHandler_GeneratePrometheusFormat(t *testing.T) {
	service := NewMetricsService(nil)
	handler := NewMetricsHandler(service)

	serviceMetrics := ServiceMetrics{
		RequestCount: 100,
		ErrorCount:   5,
	}
	systemMetrics := SystemMetrics{
		MemoryUsageMB:   256,
		CPUUsagePercent: 25.5,
		GoroutineCount:  75,
	}
	streamMetrics := map[string]interface{}{
		"total_streams":     10,
		"active_streams":    8,
		"events_processed":  5000,
		"events_per_second": 125.75,
	}

	prometheus := handler.generatePrometheusFormat(serviceMetrics, systemMetrics, streamMetrics)

	// Verify content contains expected metrics
	assert.Contains(t, prometheus, "replicator_requests_total 100")
	assert.Contains(t, prometheus, "replicator_errors_total 5")
	assert.Contains(t, prometheus, "replicator_memory_usage_bytes 268435456") // 256 * 1024 * 1024
	assert.Contains(t, prometheus, "replicator_cpu_usage_percent 25.50")
	assert.Contains(t, prometheus, "replicator_goroutines_count 75")
	assert.Contains(t, prometheus, "replicator_streams_total 10")
	assert.Contains(t, prometheus, "replicator_active_streams 8")
	assert.Contains(t, prometheus, "replicator_events_processed_total 5000")
	assert.Contains(t, prometheus, "replicator_events_per_second 125.75")

	// Verify HELP and TYPE lines are present
	assert.Contains(t, prometheus, "# HELP replicator_requests_total")
	assert.Contains(t, prometheus, "# TYPE replicator_requests_total counter")
	assert.Contains(t, prometheus, "# HELP replicator_memory_usage_bytes")
	assert.Contains(t, prometheus, "# TYPE replicator_memory_usage_bytes gauge")
}

func TestNewMetricsMiddleware(t *testing.T) {
	service := NewMetricsService(nil)
	middleware := NewMetricsMiddleware(service)

	assert.NotNil(t, middleware)
	assert.Equal(t, service, middleware.metricsService)
}

func TestMetricsMiddleware_Middleware(t *testing.T) {
	service := NewMetricsService(nil)
	middleware := NewMetricsMiddleware(service)
	
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Wrap with middleware
	wrappedHandler := middleware.Middleware(testHandler)

	// Test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	initialCount := service.requestCount

	wrappedHandler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())

	// Verify metrics were recorded
	assert.Equal(t, initialCount+1, service.requestCount)
	assert.WithinDuration(t, time.Now(), service.lastRequest, time.Second)
}

func TestMetricsMiddleware_MiddlewareWithError(t *testing.T) {
	service := NewMetricsService(nil)
	middleware := NewMetricsMiddleware(service)
	
	// Create a test handler that returns an error
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error response"))
	})

	// Wrap with middleware
	wrappedHandler := middleware.Middleware(testHandler)

	// Test request
	req := httptest.NewRequest("POST", "/test-error", nil)
	w := httptest.NewRecorder()

	initialCount := service.requestCount
	initialErrors := service.errorCount

	wrappedHandler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "error response", w.Body.String())

	// Verify metrics were recorded
	assert.Equal(t, initialCount+1, service.requestCount)
	assert.Equal(t, initialErrors+1, service.errorCount)
}

func TestResponseWriter(t *testing.T) {
	// Test response writer wrapper
	recorder := httptest.NewRecorder()
	wrapper := &responseWriter{
		ResponseWriter: recorder,
		statusCode:     http.StatusOK,
	}

	// Test WriteHeader
	wrapper.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, wrapper.statusCode)
	assert.Equal(t, http.StatusCreated, recorder.Code)

	// Test Write
	data := []byte("test data")
	n, err := wrapper.Write(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Equal(t, "test data", recorder.Body.String())
}

func TestMetricsIntegration(t *testing.T) {
	// Integration test for the entire metrics system
	telemetryManager := &metrics.TelemetryManager{}
	service := NewMetricsService(telemetryManager)
	handler := NewMetricsHandler(service)
	middleware := NewMetricsMiddleware(service)

	// Create a test application handler
	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/metrics") {
			handler.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("app response"))
	})

	// Wrap with metrics middleware
	wrappedHandler := middleware.Middleware(appHandler)

	// Make some requests to generate metrics
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Make an error request
	req := httptest.NewRequest("GET", "/api/error", nil)
	w := httptest.NewRecorder()
	// Simulate error by modifying handler
	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	errorWrapped := middleware.Middleware(errorHandler)
	errorWrapped.ServeHTTP(w, req)

	// Now request metrics
	req = httptest.NewRequest("GET", "/metrics", nil)
	w = httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Verify metrics contain expected data
	var response MetricsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Service.RequestCount >= 6) // 5 + 1 error + 1 metrics request
	assert.True(t, response.Service.ErrorCount >= 1)   // 1 error request
	assert.Equal(t, "1.0.0", response.Service.Version)
	assert.NotEmpty(t, response.Service.Uptime)
}

func BenchmarkMetricsHandler_JSON(b *testing.B) {
	service := NewMetricsService(nil)
	handler := NewMetricsHandler(service)

	req := httptest.NewRequest("GET", "/metrics", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkMetricsHandler_Prometheus(b *testing.B) {
	service := NewMetricsService(nil)
	handler := NewMetricsHandler(service)

	req := httptest.NewRequest("GET", "/metrics?format=prometheus", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkMetricsMiddleware(b *testing.B) {
	service := NewMetricsService(nil)
	middleware := NewMetricsMiddleware(service)
	
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrappedHandler := middleware.Middleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)
	}
}