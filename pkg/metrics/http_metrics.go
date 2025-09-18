package metrics

import (
	"context"
	"time"
)

// RecordHealthCheck records health check metrics (stub implementation)
func (tm *TelemetryManager) RecordHealthCheck(status string, duration time.Duration) {
	// TODO: Implement actual health check metrics recording
}

// RecordHTTPRequest records HTTP request metrics (stub implementation)
func (tm *TelemetryManager) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	// TODO: Implement actual HTTP request metrics recording
}

// RecordMetrics records arbitrary metrics (for backward compatibility)
func (tm *TelemetryManager) RecordMetrics(ctx context.Context, metrics map[string]interface{}) {
	// This method is kept for backward compatibility
	// In practice, specific metric recording methods should be preferred
}

// IncrementCounter increments a named counter (for backward compatibility)
func (tm *TelemetryManager) IncrementCounter(name string, value int64) {
	if !tm.config.Metrics.Enabled {
		return
	}
	
	ctx := context.Background()
	if counter, exists := tm.counters[name]; exists {
		counter.Add(ctx, value)
	}
}

// SetGauge sets a gauge value (placeholder - gauges in this implementation are observable)
func (tm *TelemetryManager) SetGauge(name string, value float64, labels map[string]string) {
	// Observable gauges are updated via callbacks, not directly set
	// This method is kept for backward compatibility with existing code
}
