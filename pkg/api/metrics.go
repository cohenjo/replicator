package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/cohenjo/replicator/pkg/metrics"
	"github.com/rs/zerolog/log"
)

// MetricsResponse represents the metrics API response
type MetricsResponse struct {
	Timestamp time.Time              `json:"timestamp"`
	Service   ServiceMetrics         `json:"service"`
	Streams   map[string]interface{} `json:"streams"`
	System    SystemMetrics          `json:"system"`
}

// ServiceMetrics represents service-level metrics
type ServiceMetrics struct {
	Uptime           string    `json:"uptime"`
	Version          string    `json:"version"`
	RequestCount     int64     `json:"request_count"`
	ErrorCount       int64     `json:"error_count"`
	AverageLatency   string    `json:"average_latency"`
	LastRequestTime  time.Time `json:"last_request_time,omitempty"`
	HealthStatus     string    `json:"health_status"`
}

// SystemMetrics represents system-level metrics
type SystemMetrics struct {
	MemoryUsageMB    int64   `json:"memory_usage_mb"`
	CPUUsagePercent  float64 `json:"cpu_usage_percent"`
	GoroutineCount   int     `json:"goroutine_count"`
	GCPauseMs        float64 `json:"gc_pause_ms"`
	OpenConnections  int64   `json:"open_connections"`
}

// MetricsCollector defines the interface for collecting metrics
type MetricsCollector interface {
	GetServiceMetrics() ServiceMetrics
	GetSystemMetrics() SystemMetrics
	GetStreamMetrics() map[string]interface{}
	RecordRequest(method, path string, duration time.Duration, statusCode int)
}

// MetricsService manages metrics collection and reporting
type MetricsService struct {
	telemetry    *metrics.TelemetryManager
	startTime    time.Time
	version      string
	requestCount int64
	errorCount   int64
	lastRequest  time.Time
	healthStatus string
}

// NewMetricsService creates a new metrics service
func NewMetricsService(telemetry *metrics.TelemetryManager) *MetricsService {
	return &MetricsService{
		telemetry:    telemetry,
		startTime:    time.Now(),
		version:      "1.0.0", // Should be injected at build time
		healthStatus: "healthy",
	}
}

// GetServiceMetrics returns service-level metrics
func (m *MetricsService) GetServiceMetrics() ServiceMetrics {
	return ServiceMetrics{
		Uptime:          time.Since(m.startTime).String(),
		Version:         m.version,
		RequestCount:    m.requestCount,
		ErrorCount:      m.errorCount,
		AverageLatency:  "0ms", // TODO: Calculate from telemetry
		LastRequestTime: m.lastRequest,
		HealthStatus:    m.healthStatus,
	}
}

// GetSystemMetrics returns system-level metrics
func (m *MetricsService) GetSystemMetrics() SystemMetrics {
	// TODO: Implement actual system metrics collection
	return SystemMetrics{
		MemoryUsageMB:   100,
		CPUUsagePercent: 15.5,
		GoroutineCount:  50,
		GCPauseMs:       0.5,
		OpenConnections: 10,
	}
}

// GetStreamMetrics returns stream-specific metrics
func (m *MetricsService) GetStreamMetrics() map[string]interface{} {
	// TODO: Implement actual stream metrics collection
	return map[string]interface{}{
		"total_streams":     5,
		"active_streams":    4,
		"paused_streams":    1,
		"failed_streams":    0,
		"events_processed":  12345,
		"events_per_second": 150.5,
		"total_lag_ms":      250,
	}
}

// RecordRequest records metrics for an HTTP request
func (m *MetricsService) RecordRequest(method, path string, duration time.Duration, statusCode int) {
	m.requestCount++
	m.lastRequest = time.Now()
	
	if statusCode >= 400 {
		m.errorCount++
	}
	
	// Record in telemetry if available
	if m.telemetry != nil {
		m.telemetry.RecordHTTPRequest(method, path, statusCode, duration)
	}
	
	log.Debug().
		Str("method", method).
		Str("path", path).
		Int("status_code", statusCode).
		Dur("duration", duration).
		Msg("HTTP request recorded")
}

// SetHealthStatus updates the health status
func (m *MetricsService) SetHealthStatus(status string) {
	m.healthStatus = status
}

// GetMetrics returns all metrics in a structured format
func (m *MetricsService) GetMetrics() MetricsResponse {
	return MetricsResponse{
		Timestamp: time.Now(),
		Service:   m.GetServiceMetrics(),
		Streams:   m.GetStreamMetrics(),
		System:    m.GetSystemMetrics(),
	}
}

// MetricsHandler handles HTTP metrics requests
type MetricsHandler struct {
	metricsService *MetricsService
}

// NewMetricsHandler creates a new metrics HTTP handler
func NewMetricsHandler(metricsService *MetricsService) *MetricsHandler {
	return &MetricsHandler{
		metricsService: metricsService,
	}
}

// ServeHTTP implements the http.Handler interface for metrics
func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
	// Only allow GET requests
	if r.Method != http.MethodGet {
		h.recordAndRespond(w, r, http.StatusMethodNotAllowed, startTime)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Check for format parameter
	format := r.URL.Query().Get("format")
	
	switch format {
	case "prometheus":
		h.servePrometheusMetrics(w, r, startTime)
	case "json", "":
		h.serveJSONMetrics(w, r, startTime)
	default:
		h.recordAndRespond(w, r, http.StatusBadRequest, startTime)
		http.Error(w, "Unsupported format. Use 'json' or 'prometheus'", http.StatusBadRequest)
		return
	}
}

// serveJSONMetrics serves metrics in JSON format
func (h *MetricsHandler) serveJSONMetrics(w http.ResponseWriter, r *http.Request, startTime time.Time) {
	// Get metrics
	metricsResponse := h.metricsService.GetMetrics()
	
	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	
	// Encode and send response
	if err := json.NewEncoder(w).Encode(metricsResponse); err != nil {
		log.Error().Err(err).Msg("Failed to encode metrics response")
		h.recordAndRespond(w, r, http.StatusInternalServerError, startTime)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	h.recordAndRespond(w, r, http.StatusOK, startTime)
	
	log.Debug().
		Str("format", "json").
		Str("remote_addr", r.RemoteAddr).
		Dur("duration", time.Since(startTime)).
		Msg("Metrics request completed")
}

// servePrometheusMetrics serves metrics in Prometheus format
func (h *MetricsHandler) servePrometheusMetrics(w http.ResponseWriter, r *http.Request, startTime time.Time) {
	// Get metrics
	serviceMetrics := h.metricsService.GetServiceMetrics()
	systemMetrics := h.metricsService.GetSystemMetrics()
	streamMetrics := h.metricsService.GetStreamMetrics()
	
	// Set response headers
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	
	// Generate Prometheus format
	prometheusData := h.generatePrometheusFormat(serviceMetrics, systemMetrics, streamMetrics)
	
	if _, err := w.Write([]byte(prometheusData)); err != nil {
		log.Error().Err(err).Msg("Failed to write Prometheus metrics")
		h.recordAndRespond(w, r, http.StatusInternalServerError, startTime)
		return
	}
	
	h.recordAndRespond(w, r, http.StatusOK, startTime)
	
	log.Debug().
		Str("format", "prometheus").
		Str("remote_addr", r.RemoteAddr).
		Dur("duration", time.Since(startTime)).
		Msg("Metrics request completed")
}

// generatePrometheusFormat converts metrics to Prometheus format
func (h *MetricsHandler) generatePrometheusFormat(service ServiceMetrics, system SystemMetrics, streams map[string]interface{}) string {
	var prometheus string
	
	// Service metrics
	prometheus += "# HELP replicator_requests_total Total number of HTTP requests\n"
	prometheus += "# TYPE replicator_requests_total counter\n"
	prometheus += "replicator_requests_total " + strconv.FormatInt(service.RequestCount, 10) + "\n\n"
	
	prometheus += "# HELP replicator_errors_total Total number of HTTP errors\n"
	prometheus += "# TYPE replicator_errors_total counter\n"
	prometheus += "replicator_errors_total " + strconv.FormatInt(service.ErrorCount, 10) + "\n\n"
	
	// System metrics
	prometheus += "# HELP replicator_memory_usage_bytes Memory usage in bytes\n"
	prometheus += "# TYPE replicator_memory_usage_bytes gauge\n"
	prometheus += "replicator_memory_usage_bytes " + strconv.FormatInt(system.MemoryUsageMB*1024*1024, 10) + "\n\n"
	
	prometheus += "# HELP replicator_cpu_usage_percent CPU usage percentage\n"
	prometheus += "# TYPE replicator_cpu_usage_percent gauge\n"
	prometheus += "replicator_cpu_usage_percent " + strconv.FormatFloat(system.CPUUsagePercent, 'f', 2, 64) + "\n\n"
	
	prometheus += "# HELP replicator_goroutines_count Number of goroutines\n"
	prometheus += "# TYPE replicator_goroutines_count gauge\n"
	prometheus += "replicator_goroutines_count " + strconv.Itoa(system.GoroutineCount) + "\n\n"
	
	// Stream metrics
	if totalStreams, ok := streams["total_streams"].(int); ok {
		prometheus += "# HELP replicator_streams_total Total number of streams\n"
		prometheus += "# TYPE replicator_streams_total gauge\n"
		prometheus += "replicator_streams_total " + strconv.Itoa(totalStreams) + "\n\n"
	}
	
	if activeStreams, ok := streams["active_streams"].(int); ok {
		prometheus += "# HELP replicator_active_streams Number of active streams\n"
		prometheus += "# TYPE replicator_active_streams gauge\n"
		prometheus += "replicator_active_streams " + strconv.Itoa(activeStreams) + "\n\n"
	}
	
	if eventsProcessed, ok := streams["events_processed"].(int); ok {
		prometheus += "# HELP replicator_events_processed_total Total events processed\n"
		prometheus += "# TYPE replicator_events_processed_total counter\n"
		prometheus += "replicator_events_processed_total " + strconv.Itoa(eventsProcessed) + "\n\n"
	}
	
	if eventsPerSecond, ok := streams["events_per_second"].(float64); ok {
		prometheus += "# HELP replicator_events_per_second Events processed per second\n"
		prometheus += "# TYPE replicator_events_per_second gauge\n"
		prometheus += "replicator_events_per_second " + strconv.FormatFloat(eventsPerSecond, 'f', 2, 64) + "\n\n"
	}
	
	return prometheus
}

// recordAndRespond records the request and response metrics
func (h *MetricsHandler) recordAndRespond(w http.ResponseWriter, r *http.Request, statusCode int, startTime time.Time) {
	duration := time.Since(startTime)
	h.metricsService.RecordRequest(r.Method, r.URL.Path, duration, statusCode)
}

// MetricsMiddleware provides HTTP request metrics middleware
type MetricsMiddleware struct {
	metricsService *MetricsService
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(metricsService *MetricsService) *MetricsMiddleware {
	return &MetricsMiddleware{
		metricsService: metricsService,
	}
}

// Middleware returns an HTTP middleware function
func (m *MetricsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		
		// Create response writer wrapper to capture status code
		wrapper := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		
		// Call next handler
		next.ServeHTTP(wrapper, r)
		
		// Record metrics
		duration := time.Since(startTime)
		m.metricsService.RecordRequest(r.Method, r.URL.Path, duration, wrapper.statusCode)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write ensures status code is set if not already set
func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}