package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/metrics"
	"github.com/rs/zerolog/log"
)

// HealthStatus represents the overall health status of the service
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status      HealthStatus           `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	Version     string                 `json:"version"`
	Uptime      string                 `json:"uptime"`
	Checks      map[string]CheckResult `json:"checks"`
	Environment string                 `json:"environment,omitempty"`
}

// CheckResult represents the result of a single health check
type CheckResult struct {
	Status      HealthStatus  `json:"status"`
	Message     string        `json:"message,omitempty"`
	Error       string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
	Timestamp   time.Time     `json:"timestamp"`
	ComponentID string        `json:"component_id,omitempty"`
}

// HealthChecker defines the interface for health check implementations
type HealthChecker interface {
	Name() string
	Check() CheckResult
	IsEssential() bool
}

// HealthService manages health checks for the replication service
type HealthService struct {
	checkers   []HealthChecker
	startTime  time.Time
	version    string
	cfg        *config.Config
	telemetry  *metrics.TelemetryManager
}

// NewHealthService creates a new health service
func NewHealthService(cfg *config.Config, telemetry *metrics.TelemetryManager) *HealthService {
	return &HealthService{
		checkers:  make([]HealthChecker, 0),
		startTime: time.Now(),
		version:   "1.0.0", // Should be injected at build time
		cfg:       cfg,
		telemetry: telemetry,
	}
}

// RegisterChecker adds a health checker to the service
func (h *HealthService) RegisterChecker(checker HealthChecker) {
	h.checkers = append(h.checkers, checker)
	log.Debug().Str("checker", checker.Name()).Msg("Health checker registered")
}

// PerformHealthCheck executes all registered health checks
func (h *HealthService) PerformHealthCheck() HealthResponse {
	startTime := time.Now()
	checks := make(map[string]CheckResult)
	overallStatus := HealthStatusHealthy
	
	// Execute all health checks
	for _, checker := range h.checkers {
		checkStart := time.Now()
		result := checker.Check()
		result.Duration = time.Since(checkStart)
		result.Timestamp = time.Now()
		
		checks[checker.Name()] = result
		
		// Determine overall status based on check results
		if result.Status == HealthStatusUnhealthy && checker.IsEssential() {
			overallStatus = HealthStatusUnhealthy
		} else if result.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
			overallStatus = HealthStatusDegraded
		}
		
		log.Debug().
			Str("checker", checker.Name()).
			Str("status", string(result.Status)).
			Dur("duration", result.Duration).
			Msg("Health check completed")
	}
	
	// Record health check metrics
	if h.telemetry != nil {
		h.telemetry.RecordHealthCheck(string(overallStatus), time.Since(startTime))
	}
	
	return HealthResponse{
		Status:      overallStatus,
		Timestamp:   time.Now(),
		Version:     h.version,
		Uptime:      time.Since(h.startTime).String(),
		Checks:      checks,
		Environment: h.getEnvironment(),
	}
}

// getEnvironment returns the current environment (dev, staging, prod)
func (h *HealthService) getEnvironment() string {
	if h.cfg != nil && h.cfg.Logging.Level == "debug" {
		return "development"
	}
	return "production"
}

// HealthHandler handles HTTP health check requests
type HealthHandler struct {
	healthService *HealthService
}

// NewHealthHandler creates a new health check HTTP handler
func NewHealthHandler(healthService *HealthService) *HealthHandler {
	return &HealthHandler{
		healthService: healthService,
	}
}

// ServeHTTP implements the http.Handler interface for health checks
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Perform health check
	healthResponse := h.healthService.PerformHealthCheck()
	
	// Set appropriate HTTP status code based on health status
	var statusCode int
	switch healthResponse.Status {
	case HealthStatusHealthy:
		statusCode = http.StatusOK
	case HealthStatusDegraded:
		statusCode = http.StatusOK // Still return 200 for degraded but functional
	case HealthStatusUnhealthy:
		statusCode = http.StatusServiceUnavailable
	default:
		statusCode = http.StatusInternalServerError
	}
	
	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(statusCode)
	
	// Encode and send response
	if err := json.NewEncoder(w).Encode(healthResponse); err != nil {
		log.Error().Err(err).Msg("Failed to encode health response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	log.Debug().
		Str("status", string(healthResponse.Status)).
		Int("status_code", statusCode).
		Str("remote_addr", r.RemoteAddr).
		Str("user_agent", r.UserAgent()).
		Msg("Health check request completed")
}

// Built-in health checkers

// DatabaseChecker checks database connectivity
type DatabaseChecker struct {
	name        string
	essential   bool
	pingFunc    func() error
	componentID string
}

// NewDatabaseChecker creates a new database health checker
func NewDatabaseChecker(name string, essential bool, pingFunc func() error, componentID string) *DatabaseChecker {
	return &DatabaseChecker{
		name:        name,
		essential:   essential,
		pingFunc:    pingFunc,
		componentID: componentID,
	}
}

// Name returns the checker name
func (d *DatabaseChecker) Name() string {
	return d.name
}

// IsEssential returns whether this check is essential
func (d *DatabaseChecker) IsEssential() bool {
	return d.essential
}

// Check performs the database connectivity check
func (d *DatabaseChecker) Check() CheckResult {
	if d.pingFunc == nil {
		return CheckResult{
			Status:      HealthStatusUnhealthy,
			Error:       "ping function not configured",
			ComponentID: d.componentID,
		}
	}
	
	if err := d.pingFunc(); err != nil {
		return CheckResult{
			Status:      HealthStatusUnhealthy,
			Error:       err.Error(),
			ComponentID: d.componentID,
		}
	}
	
	return CheckResult{
		Status:      HealthStatusHealthy,
		Message:     "database connection successful",
		ComponentID: d.componentID,
	}
}

// StreamChecker checks the status of replication streams
type StreamChecker struct {
	name           string
	essential      bool
	getStreamCount func() (active, total int, err error)
	componentID    string
}

// NewStreamChecker creates a new stream health checker
func NewStreamChecker(name string, essential bool, getStreamCount func() (active, total int, err error), componentID string) *StreamChecker {
	return &StreamChecker{
		name:           name,
		essential:      essential,
		getStreamCount: getStreamCount,
		componentID:    componentID,
	}
}

// Name returns the checker name
func (s *StreamChecker) Name() string {
	return s.name
}

// IsEssential returns whether this check is essential
func (s *StreamChecker) IsEssential() bool {
	return s.essential
}

// Check performs the stream status check
func (s *StreamChecker) Check() CheckResult {
	if s.getStreamCount == nil {
		return CheckResult{
			Status:      HealthStatusUnhealthy,
			Error:       "stream count function not configured",
			ComponentID: s.componentID,
		}
	}
	
	active, total, err := s.getStreamCount()
	if err != nil {
		return CheckResult{
			Status:      HealthStatusUnhealthy,
			Error:       err.Error(),
			ComponentID: s.componentID,
		}
	}
	
	// Determine status based on stream health
	status := HealthStatusHealthy
	message := "all streams healthy"
	
	if active == 0 && total > 0 {
		status = HealthStatusUnhealthy
		message = "no active streams"
	} else if active < total {
		status = HealthStatusDegraded
		message = "some streams inactive"
	}
	
	return CheckResult{
		Status:      status,
		Message:     message,
		ComponentID: s.componentID,
	}
}

// MemoryChecker checks memory usage
type MemoryChecker struct {
	name         string
	essential    bool
	maxMemoryMB  int64
	getMemoryMB  func() (int64, error)
	componentID  string
}

// NewMemoryChecker creates a new memory health checker
func NewMemoryChecker(name string, essential bool, maxMemoryMB int64, getMemoryMB func() (int64, error), componentID string) *MemoryChecker {
	return &MemoryChecker{
		name:         name,
		essential:    essential,
		maxMemoryMB:  maxMemoryMB,
		getMemoryMB:  getMemoryMB,
		componentID:  componentID,
	}
}

// Name returns the checker name
func (m *MemoryChecker) Name() string {
	return m.name
}

// IsEssential returns whether this check is essential
func (m *MemoryChecker) IsEssential() bool {
	return m.essential
}

// Check performs the memory usage check
func (m *MemoryChecker) Check() CheckResult {
	if m.getMemoryMB == nil {
		return CheckResult{
			Status:      HealthStatusUnhealthy,
			Error:       "memory function not configured",
			ComponentID: m.componentID,
		}
	}
	
	currentMB, err := m.getMemoryMB()
	if err != nil {
		return CheckResult{
			Status:      HealthStatusUnhealthy,
			Error:       err.Error(),
			ComponentID: m.componentID,
		}
	}
	
	// Check memory thresholds
	status := HealthStatusHealthy
	message := "memory usage normal"
	
	if currentMB > m.maxMemoryMB {
		status = HealthStatusUnhealthy
		message = "memory usage exceeded limit"
	} else if currentMB > m.maxMemoryMB*8/10 { // 80% threshold
		status = HealthStatusDegraded
		message = "memory usage high"
	}
	
	return CheckResult{
		Status:      status,
		Message:     message,
		ComponentID: m.componentID,
	}
}