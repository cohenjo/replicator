package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/metrics"
	"github.com/rs/zerolog/log"
)

// ConfigManager represents configuration management interface
type ConfigManager interface {
	GetConfig() *config.Config
	ReloadConfig(ctx context.Context) error
	UpdateStream(name string, config config.StreamConfig) error
	ValidateConfig(config *config.Config) error
}

// DefaultConfigManager implements ConfigManager
type DefaultConfigManager struct {
	configService ConfigService
}

// NewDefaultConfigManager creates a new default config manager
func NewDefaultConfigManager(configService ConfigService) *DefaultConfigManager {
	return &DefaultConfigManager{
		configService: configService,
	}
}

// GetConfig returns the current configuration
func (cm *DefaultConfigManager) GetConfig() *config.Config {
	cfg, _ := cm.configService.GetConfig()
	return cfg
}

// ReloadConfig reloads configuration
func (cm *DefaultConfigManager) ReloadConfig(ctx context.Context) error {
	_, err := cm.configService.ReloadConfig()
	return err
}

// UpdateStream updates a stream configuration
func (cm *DefaultConfigManager) UpdateStream(name string, streamConfig config.StreamConfig) error {
	// Implementation would update the specific stream config
	return nil
}

// ValidateConfig validates configuration
func (cm *DefaultConfigManager) ValidateConfig(cfg *config.Config) error {
	return cfg.Validate()
}

// Server represents the main HTTP server
type Server struct {
	config         *config.Config
	httpServer     *http.Server
	healthService  *HealthService
	metricsService *MetricsService
	streamService  StreamManager
	configService  ConfigManager

	// Handlers
	healthHandler  *HealthHandler
	metricsHandler *MetricsHandler
	streamsHandler *StreamsHandler
	configHandler  *ConfigHandler

	// Middleware
	metricsMiddleware *MetricsMiddleware
}

// ServerConfig represents configuration for the HTTP server
type ServerConfig struct {
	Host           string        `json:"host"`
	Port           int           `json:"port"`
	ReadTimeout    time.Duration `json:"read_timeout"`
	WriteTimeout   time.Duration `json:"write_timeout"`
	IdleTimeout    time.Duration `json:"idle_timeout"`
	EnableTLS      bool          `json:"enable_tls"`
	TLSCertFile    string        `json:"tls_cert_file,omitempty"`
	TLSKeyFile     string        `json:"tls_key_file,omitempty"`
	EnableCORS     bool          `json:"enable_cors"`
	CORSOrigins    []string      `json:"cors_origins,omitempty"`
	EnableMetrics  bool          `json:"enable_metrics"`
	EnableAuth     bool          `json:"enable_auth"`
	AuthTokens     []string      `json:"auth_tokens,omitempty"`
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:          "0.0.0.0",
		Port:          8080,
		ReadTimeout:   30 * time.Second,
		WriteTimeout:  30 * time.Second,
		IdleTimeout:   60 * time.Second,
		EnableTLS:     false,
		EnableCORS:    true,
		CORSOrigins:   []string{"*"},
		EnableMetrics: true,
		EnableAuth:    false,
	}
}

// NewServer creates a new HTTP API server
func NewServer(
	cfg *config.Config,
	serverCfg ServerConfig,
	telemetry *metrics.TelemetryManager,
	streamRunner StreamRunner,
) (*Server, error) {
		// Create health service
	healthService := NewHealthService(cfg, telemetry)
	metricsService := NewMetricsService(telemetry)
	streamService := NewStreamService(cfg, streamRunner)
	configService := NewConfigService(cfg, "")

	// Create handlers
	healthHandler := NewHealthHandler(healthService)
	metricsHandler := NewMetricsHandler(metricsService)
	streamsHandler := NewStreamsHandler(streamService)
	configHandler := NewConfigHandler(configService)

	// Create middleware
	metricsMiddleware := NewMetricsMiddleware(metricsService)

	// Create server
	server := &Server{
		config:            cfg,
		healthService:     healthService,
		metricsService:    metricsService,
		streamService:     streamService,
		configService:     NewDefaultConfigManager(configService),
		healthHandler:     healthHandler,
		metricsHandler:    metricsHandler,
		streamsHandler:    streamsHandler,
		configHandler:     configHandler,
		metricsMiddleware: metricsMiddleware,
	}

	// Setup HTTP server
	mux := server.createMux(serverCfg)
	
	server.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", serverCfg.Host, serverCfg.Port),
		Handler:      mux,
		ReadTimeout:  serverCfg.ReadTimeout,
		WriteTimeout: serverCfg.WriteTimeout,
		IdleTimeout:  serverCfg.IdleTimeout,
	}

	log.Info().
		Str("address", server.httpServer.Addr).
		Bool("tls_enabled", serverCfg.EnableTLS).
		Bool("cors_enabled", serverCfg.EnableCORS).
		Bool("metrics_enabled", serverCfg.EnableMetrics).
		Bool("auth_enabled", serverCfg.EnableAuth).
		Msg("HTTP API server created")

	return server, nil
}

// createMux creates the HTTP multiplexer with all routes and middleware
func (s *Server) createMux(serverCfg ServerConfig) http.Handler {
	mux := http.NewServeMux()

	// Health endpoints
	mux.Handle("/health", s.healthHandler)
	mux.Handle("/health/", s.healthHandler)

	// Metrics endpoints (if enabled)
	if serverCfg.EnableMetrics {
		mux.Handle("/metrics", s.metricsHandler)
		mux.Handle("/metrics/", s.metricsHandler)
	}

	// API endpoints
	mux.Handle("/api/v1/streams", s.streamsHandler)
	mux.Handle("/api/v1/streams/", s.streamsHandler)
	mux.Handle("/api/v1/config", s.configHandler)
	mux.Handle("/api/v1/config/", s.configHandler)

	// Legacy endpoints (without /api/v1 prefix)
	mux.Handle("/streams", s.streamsHandler)
	mux.Handle("/streams/", s.streamsHandler)
	mux.Handle("/config", s.configHandler)
	mux.Handle("/config/", s.configHandler)

	// Root endpoint - API information
	mux.HandleFunc("/", s.handleRoot)

	// API documentation endpoint
	mux.HandleFunc("/api", s.handleAPIInfo)
	mux.HandleFunc("/api/", s.handleAPIInfo)

	// Wrap with middleware
	var handler http.Handler = mux

	// Add metrics middleware
	if serverCfg.EnableMetrics {
		handler = s.metricsMiddleware.Middleware(handler)
	}

	// Add CORS middleware
	if serverCfg.EnableCORS {
		handler = s.corsMiddleware(handler, serverCfg.CORSOrigins)
	}

	// Add auth middleware
	if serverCfg.EnableAuth {
		handler = s.authMiddleware(handler, serverCfg.AuthTokens)
	}

	// Add logging middleware
	handler = s.loggingMiddleware(handler)

	// Add recovery middleware
	handler = s.recoveryMiddleware(handler)

	return handler
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Info().
		Str("address", s.httpServer.Addr).
		Msg("Starting HTTP API server")

	return s.httpServer.ListenAndServe()
}

// StartTLS starts the HTTP server with TLS
func (s *Server) StartTLS(certFile, keyFile string) error {
	log.Info().
		Str("address", s.httpServer.Addr).
		Str("cert_file", certFile).
		Str("key_file", keyFile).
		Msg("Starting HTTPS API server")

	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	log.Info().Msg("Stopping HTTP API server")
	return s.httpServer.Shutdown(ctx)
}

// GetAddr returns the server address
func (s *Server) GetAddr() string {
	return s.httpServer.Addr
}

// IsRunning returns true if the server is running
func (s *Server) IsRunning() bool {
	// Simple check - in production, might want to implement proper health checking
	return s.httpServer != nil
}

// handleRoot handles the root endpoint
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	response := map[string]interface{}{
		"service":     "replicator-api",
		"version":     "1.0.0", // TODO: Get from build info
		"status":      "running",
		"timestamp":   time.Now(),
		"endpoints": map[string]interface{}{
			"health":  "/health",
			"metrics": "/metrics",
			"api":     "/api",
			"streams": "/api/v1/streams",
			"config":  "/api/v1/config",
		},
		"documentation": "/api",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode root response")
	}
}

// handleAPIInfo handles the API information endpoint
func (s *Server) handleAPIInfo(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"api_version": "v1",
		"service":     "replicator-api",
		"endpoints": map[string]interface{}{
			"health": map[string]interface{}{
				"path":        "/health",
				"methods":     []string{"GET"},
				"description": "Service health check with component status",
			},
			"metrics": map[string]interface{}{
				"path":        "/metrics",
				"methods":     []string{"GET"},
				"description": "Service metrics in JSON or Prometheus format",
				"parameters":  map[string]string{"format": "json|prometheus"},
			},
			"streams": map[string]interface{}{
				"path":        "/api/v1/streams",
				"methods":     []string{"GET", "POST"},
				"description": "Manage replication streams",
				"endpoints": map[string]interface{}{
					"list":    "GET /api/v1/streams",
					"create":  "POST /api/v1/streams",
					"get":     "GET /api/v1/streams/{id}",
					"update":  "PUT /api/v1/streams/{id}",
					"delete":  "DELETE /api/v1/streams/{id}",
					"actions": "POST /api/v1/streams/{id}/actions",
					"metrics": "GET /api/v1/streams/{id}/metrics",
				},
			},
			"config": map[string]interface{}{
				"path":        "/api/v1/config",
				"methods":     []string{"GET", "PUT"},
				"description": "Manage system configuration",
				"endpoints": map[string]interface{}{
					"get":      "GET /api/v1/config",
					"update":   "PUT /api/v1/config",
					"reload":   "POST /api/v1/config/reload",
					"validate": "POST /api/v1/config/validate",
					"backup":   "POST /api/v1/config/backup",
					"backups":  "GET /api/v1/config/backups",
					"restore":  "POST /api/v1/config/backups/{id}/restore",
				},
			},
		},
		"authentication": map[string]interface{}{
			"required": false, // TODO: Get from config
			"type":     "Bearer Token",
		},
		"rate_limiting": map[string]interface{}{
			"enabled": false, // TODO: Implement rate limiting
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode API info response")
	}
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		
		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware adds authentication validation
func (s *Server) authMiddleware(next http.Handler, validTokens []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health and metrics endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Check Bearer token format
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		
		// Validate token
		valid := false
		for _, validToken := range validTokens {
			if token == validToken {
				valid = true
				break
			}
		}

		if !valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		
		// Create a custom response writer to capture status code
		wrapped := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call next handler
		next.ServeHTTP(wrapped, r)

		// Log request
		duration := time.Since(startTime)
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.Header.Get("User-Agent")).
			Int("status_code", wrapped.statusCode).
			Dur("duration", duration).
			Msg("HTTP request")
	})
}

// recoveryMiddleware recovers from panics
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Error().
					Interface("error", err).
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("remote_addr", r.RemoteAddr).
					Msg("Panic recovered in HTTP handler")

				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// responseWriterWrapper wraps http.ResponseWriter to capture status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write ensures status code is set if not already set
func (rw *responseWriterWrapper) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}

// ServerManager manages multiple server instances
type ServerManager struct {
	servers map[string]*Server
}

// NewServerManager creates a new server manager
func NewServerManager() *ServerManager {
	return &ServerManager{
		servers: make(map[string]*Server),
	}
}

// AddServer adds a server to the manager
func (sm *ServerManager) AddServer(name string, server *Server) {
	sm.servers[name] = server
}

// GetServer returns a server by name
func (sm *ServerManager) GetServer(name string) (*Server, bool) {
	server, exists := sm.servers[name]
	return server, exists
}

// StartAll starts all managed servers
func (sm *ServerManager) StartAll() error {
	for name, server := range sm.servers {
		go func(name string, server *Server) {
			if err := server.Start(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Str("server", name).Msg("Server failed to start")
			}
		}(name, server)
	}
	return nil
}

// StopAll stops all managed servers
func (sm *ServerManager) StopAll(ctx context.Context) error {
	for name, server := range sm.servers {
		if err := server.Stop(ctx); err != nil {
			log.Error().Err(err).Str("server", name).Msg("Failed to stop server")
		}
	}
	return nil
}

// GetServerStatus returns the status of all servers
func (sm *ServerManager) GetServerStatus() map[string]bool {
	status := make(map[string]bool)
	for name, server := range sm.servers {
		status[name] = server.IsRunning()
	}
	return status
}