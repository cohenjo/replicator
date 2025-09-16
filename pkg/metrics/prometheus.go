package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// LegacyMetricsServer provides a fallback HTTP metrics endpoint
// This is kept for backward compatibility but uses OpenTelemetry internally
type LegacyMetricsServer struct {
	server    *http.Server
	telemetry *TelemetryManager
}

// NewLegacyMetricsServer creates a new legacy metrics server
func NewLegacyMetricsServer(port string, telemetry *TelemetryManager) *LegacyMetricsServer {
	mux := http.NewServeMux()
	
	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})
	
	// Metrics endpoint - returns info about OTLP
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# Metrics are exported via OpenTelemetry OTLP to the collector\n# Check the otel-collector logs or Prometheus endpoint at port 8889\n"))
	})

	return &LegacyMetricsServer{
		server: &http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
		telemetry: telemetry,
	}
}

// Start starts the legacy metrics server
func (lms *LegacyMetricsServer) Start() error {
	log.Info().
		Str("addr", lms.server.Addr).
		Msg("Starting legacy metrics server (OTLP-backed)")
	
	return lms.server.ListenAndServe()
}

// Stop stops the legacy metrics server
func (lms *LegacyMetricsServer) Stop(ctx context.Context) error {
	log.Info().Msg("Stopping legacy metrics server")
	return lms.server.Shutdown(ctx)
}

// SetupMetrics is kept for backward compatibility but now uses OTLP
func SetupMetrics() {
	log.Warn().Msg("SetupMetrics is deprecated - use TelemetryManager with OTLP instead")
}
