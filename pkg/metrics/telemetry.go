package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"github.com/rs/zerolog/log"
)

// TelemetryConfig is an alias to the config package TelemetryConfig for compatibility
type TelemetryConfig = config.TelemetryConfig

// TelemetryManager manages OpenTelemetry metrics and tracing
type TelemetryManager struct {
	config          TelemetryConfig
	meterProvider   *sdkmetric.MeterProvider
	tracerProvider  trace.TracerProvider
	meter           metric.Meter
	tracer          trace.Tracer
	
	// Metrics instruments
	counters        map[string]metric.Int64Counter
	gauges          map[string]metric.Float64ObservableGauge
	histograms      map[string]metric.Float64Histogram
	
	// Stream metrics storage
	streamMetrics   map[string]*models.ReplicationMetrics
	
	mutex           sync.RWMutex
	started         bool
}

// MetricsCollector collects and reports metrics for streams
type MetricsCollector struct {
	telemetry   *TelemetryManager
	streams     map[string]models.Stream
	collectors  map[string]*StreamCollector
	ticker      *time.Ticker
	stopChan    chan struct{}
	mutex       sync.RWMutex
}

// StreamCollector collects metrics for a specific stream
type StreamCollector struct {
	streamName    string
	stream        models.Stream
	lastMetrics   models.ReplicationMetrics
	telemetry     *TelemetryManager
	mutex         sync.RWMutex
}

// NewTelemetryManager creates a new telemetry manager
func NewTelemetryManager(config TelemetryConfig) (*TelemetryManager, error) {
	log.Info().
		Bool("enabled", config.Enabled).
		Bool("metrics_enabled", config.Metrics.Enabled).
		Bool("tracing_enabled", config.Tracing.Enabled).
		Str("otlp_endpoint", config.Metrics.OpenTelemetry.Endpoint).
		Str("service_name", config.ServiceName).
		Msg("Creating telemetry manager with config")

	tm := &TelemetryManager{
		config:        config,
		counters:      make(map[string]metric.Int64Counter),
		gauges:        make(map[string]metric.Float64ObservableGauge),
		histograms:    make(map[string]metric.Float64Histogram),
		streamMetrics: make(map[string]*models.ReplicationMetrics),
	}

	if err := tm.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize telemetry: %w", err)
	}

	return tm, nil
}

// initialize sets up OpenTelemetry
func (tm *TelemetryManager) initialize() error {
	if !tm.config.Enabled {
		log.Info().Msg("Telemetry disabled")
		return nil
	}

	// Setup metrics
	if tm.config.Metrics.Enabled {
		if err := tm.setupMetrics(); err != nil {
			return fmt.Errorf("failed to setup metrics: %w", err)
		}
	}

	// Setup tracing
	if tm.config.Tracing.Enabled {
		if err := tm.setupTracing(); err != nil {
			return fmt.Errorf("failed to setup tracing: %w", err)
		}
	}

	return tm.createInstruments()
}

// setupMetrics configures OpenTelemetry metrics with OTLP gRPC exporter
func (tm *TelemetryManager) setupMetrics() error {
	// Create OTLP gRPC exporter
	exporter, err := otlpmetricgrpc.New(
		context.Background(),
		otlpmetricgrpc.WithEndpoint(tm.config.Metrics.OpenTelemetry.Endpoint),
		otlpmetricgrpc.WithInsecure(), // Use insecure for local development
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
	}

	// Create meter provider with periodic reader
	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(tm.config.Metrics.Interval),
	)

	tm.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(tm.createResource()),
	)

	// Set global meter provider
	otel.SetMeterProvider(tm.meterProvider)

	// Create meter
	tm.meter = tm.meterProvider.Meter(
		tm.config.ServiceName,
		metric.WithInstrumentationVersion(tm.config.ServiceVersion),
	)

	log.Info().
		Str("otlp_endpoint", tm.config.Metrics.OpenTelemetry.Endpoint).
		Dur("metrics_interval", tm.config.Metrics.Interval).
		Msg("OpenTelemetry metrics configured with OTLP gRPC exporter")

	return nil
}

// setupTracing configures OpenTelemetry tracing
func (tm *TelemetryManager) setupTracing() error {
	// For now, use a no-op tracer
	// In production, you would configure a real tracer with exporters
	tm.tracerProvider = trace.NewNoopTracerProvider()
	otel.SetTracerProvider(tm.tracerProvider)
	
	tm.tracer = tm.tracerProvider.Tracer(
		tm.config.ServiceName,
		trace.WithInstrumentationVersion(tm.config.ServiceVersion),
	)
	
	return nil
}

// createResource creates an OpenTelemetry resource
func (tm *TelemetryManager) createResource() *resource.Resource {
	attributes := []attribute.KeyValue{
		attribute.String("service.name", tm.config.ServiceName),
		attribute.String("service.version", tm.config.ServiceVersion),
		attribute.String("environment", tm.config.Environment),
	}

	// Add custom labels
	for key, value := range tm.config.Labels {
		attributes = append(attributes, attribute.String(key, value))
	}

	return resource.NewWithAttributes(
		semconv.SchemaURL,
		attributes...,
	)
}

// createInstruments creates all the metric instruments
func (tm *TelemetryManager) createInstruments() error {
	if !tm.config.Metrics.Enabled || tm.meter == nil {
		return nil
	}

	var err error

	// Counters
	tm.counters["events_processed"], err = tm.meter.Int64Counter(
		"replicator_events_processed_total",
		metric.WithDescription("Total number of events processed"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create events_processed counter: %w", err)
	}

	tm.counters["events_failed"], err = tm.meter.Int64Counter(
		"replicator_events_failed_total",
		metric.WithDescription("Total number of events that failed processing"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create events_failed counter: %w", err)
	}

	tm.counters["bytes_processed"], err = tm.meter.Int64Counter(
"replicator_bytes_processed_total",
		metric.WithDescription("Total number of bytes processed"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create bytes_processed counter: %w", err)
	}

	// MongoDB-specific recovery mode counters
	tm.counters["mongodb_events_full_document"], err = tm.meter.Int64Counter(
"replicator_mongodb_events_full_document_total",
		metric.WithDescription("Total number of MongoDB events with full document (normal mode)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create mongodb_events_full_document counter: %w", err)
	}
	
	tm.counters["mongodb_events_fallback_used"], err = tm.meter.Int64Counter(
"replicator_mongodb_events_fallback_used_total",
		metric.WithDescription("Total number of MongoDB events using fallback document fetch"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create mongodb_events_fallback_used counter: %w", err)
	}
	
	tm.counters["mongodb_events_empty_payload"], err = tm.meter.Int64Counter(
"replicator_mongodb_events_empty_payload_total",
		metric.WithDescription("Total number of MongoDB events using empty payload fallback"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create mongodb_events_empty_payload counter: %w", err)
	}
	
	tm.counters["mongodb_events_fallback_failed"], err = tm.meter.Int64Counter(
"replicator_mongodb_events_fallback_failed_total",
		metric.WithDescription("Total number of MongoDB fallback document fetch failures"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create mongodb_events_fallback_failed counter: %w", err)
	}
	
	// Histograms
	tm.histograms["replication_lag"], err = tm.meter.Float64Histogram(
		"replicator_replication_lag_seconds",
		metric.WithDescription("Replication lag in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create replication_lag histogram: %w", err)
	}

	tm.histograms["processing_duration"], err = tm.meter.Float64Histogram(
		"replicator_processing_duration_seconds",
		metric.WithDescription("Event processing duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create processing_duration histogram: %w", err)
	}

	// Observable gauges
	tm.gauges["active_streams"], err = tm.meter.Float64ObservableGauge(
		"replicator_active_streams",
		metric.WithDescription("Number of active replication streams"),
		metric.WithUnit("1"),
		metric.WithFloat64Callback(tm.getActiveStreamsCount),
	)
	if err != nil {
		return fmt.Errorf("failed to create active_streams gauge: %w", err)
	}

	tm.gauges["events_per_second"], err = tm.meter.Float64ObservableGauge(
		"replicator_events_per_second",
		metric.WithDescription("Current events per second rate"),
		metric.WithUnit("1/s"),
		metric.WithFloat64Callback(tm.getEventsPerSecond),
	)
	if err != nil {
		return fmt.Errorf("failed to create events_per_second gauge: %w", err)
	}

	return nil
}

// Start starts the telemetry manager
func (tm *TelemetryManager) Start(ctx context.Context) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.started {
		return fmt.Errorf("telemetry manager already started")
	}

	if !tm.config.Enabled {
		log.Info().Msg("Telemetry disabled, skipping start")
		return nil
	}

	tm.started = true
	log.Info().Msg("Telemetry manager started")
	return nil
}

// Stop stops the telemetry manager
func (tm *TelemetryManager) Stop(ctx context.Context) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	if !tm.started {
		return nil
	}
	
	if tm.meterProvider != nil {
		if err := tm.meterProvider.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to shutdown meter provider")
		}
	}
	
	tm.started = false
	log.Info().Msg("Telemetry manager stopped")
	return nil
}

// RecordEvent records metrics for a processed event
func (tm *TelemetryManager) RecordEvent(ctx context.Context, streamName string, event models.ChangeEvent, processingTime time.Duration, success bool) {
	if !tm.config.Metrics.Enabled {
		return
	}
	
	attributes := []attribute.KeyValue{
		attribute.String("stream_name", streamName),
		attribute.String("operation_type", event.OperationType),
		attribute.Bool("success", success),
	}
	
	// Record event counter
	if success {
		tm.counters["events_processed"].Add(ctx, 1, metric.WithAttributes(attributes...))
		} else {
			tm.counters["events_failed"].Add(ctx, 1, metric.WithAttributes(attributes...))
		}
		
		// Record processing duration
		tm.histograms["processing_duration"].Record(ctx, processingTime.Seconds(), metric.WithAttributes(attributes...))
		
		// Calculate and record replication lag
		if !event.Timestamp.IsZero() {
			lag := time.Since(event.Timestamp)
			tm.histograms["replication_lag"].Record(ctx, lag.Seconds(), metric.WithAttributes(attributes...))
		}
	}
	
// RecordBytes records bytes processed
func (tm *TelemetryManager) RecordBytes(ctx context.Context, streamName string, bytes int64) {
	if !tm.config.Metrics.Enabled {
		return
	}
		
		attributes := []attribute.KeyValue{
			attribute.String("stream_name", streamName),
		}
		
		tm.counters["bytes_processed"].Add(ctx, bytes, metric.WithAttributes(attributes...))
	}
	
// RecordMongoRecoveryMode records MongoDB recovery mode metrics
func (tm *TelemetryManager) RecordMongoRecoveryMode(ctx context.Context, streamName, operation, recoveryMode string) {
	if !tm.config.Metrics.Enabled {
		return
	}
		
		attributes := []attribute.KeyValue{
			attribute.String("stream_name", streamName),
			attribute.String("operation_type", operation),
		}
		
		switch recoveryMode {
		case "normal":
			tm.counters["mongodb_events_full_document"].Add(ctx, 1, metric.WithAttributes(attributes...))
		case "fallback":
			tm.counters["mongodb_events_fallback_used"].Add(ctx, 1, metric.WithAttributes(attributes...))
		case "empty":
			tm.counters["mongodb_events_empty_payload"].Add(ctx, 1, metric.WithAttributes(attributes...))
		}
	}
	
// RecordMongoFallbackFailure records MongoDB fallback fetch failures
func (tm *TelemetryManager) RecordMongoFallbackFailure(ctx context.Context, streamName, operation string) {
	if !tm.config.Metrics.Enabled {
		return
	}
		
		attributes := []attribute.KeyValue{
			attribute.String("stream_name", streamName),
			attribute.String("operation_type", operation),
		}
		
		tm.counters["mongodb_events_fallback_failed"].Add(ctx, 1, metric.WithAttributes(attributes...))
	}
	
	
	// UpdateStreamMetrics updates metrics for a specific stream
	func (tm *TelemetryManager) UpdateStreamMetrics(streamName string, metrics models.ReplicationMetrics) {
		tm.mutex.Lock()
		defer tm.mutex.Unlock()
		
		tm.streamMetrics[streamName] = &metrics
	}
	
	// GetStreamMetrics returns metrics for a specific stream
	func (tm *TelemetryManager) GetStreamMetrics(streamName string) (*models.ReplicationMetrics, bool) {
		tm.mutex.RLock()
		defer tm.mutex.RUnlock()
		
		metrics, exists := tm.streamMetrics[streamName]
		if !exists {
			return nil, false
		}
		
		// Return a copy
		metricsCopy := *metrics
		return &metricsCopy, true
	}
	
	// GetAllStreamMetrics returns metrics for all streams
	func (tm *TelemetryManager) GetAllStreamMetrics() map[string]models.ReplicationMetrics {
		tm.mutex.RLock()
		defer tm.mutex.RUnlock()
		
		result := make(map[string]models.ReplicationMetrics)
		for name, metrics := range tm.streamMetrics {
			result[name] = *metrics
		}
		return result
	}
	
// StartTrace starts a new trace span
func (tm *TelemetryManager) StartTrace(ctx context.Context, operationName string, attributes ...attribute.KeyValue) (context.Context, trace.Span) {
	if !tm.config.Tracing.Enabled || tm.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
		
	return tm.tracer.Start(ctx, operationName, trace.WithAttributes(attributes...))
}
	
	// Observable gauge callback functions
	func (tm *TelemetryManager) getActiveStreamsCount(ctx context.Context, observer metric.Float64Observer) error {
		// Use a separate goroutine to avoid deadlock during shutdown
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		tm.mutex.RLock()
		count := float64(len(tm.streamMetrics))
		tm.mutex.RUnlock()
		
		observer.Observe(count)
		return nil
	}
	
	func (tm *TelemetryManager) getEventsPerSecond(ctx context.Context, observer metric.Float64Observer) error {
		// Use a separate goroutine to avoid deadlock during shutdown
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		tm.mutex.RLock()
		totalEPS := float64(0)
		for streamName, metrics := range tm.streamMetrics {
			observer.Observe(metrics.EventsPerSecond, metric.WithAttributes(
				attribute.String("stream_name", streamName),
				))
				totalEPS += metrics.EventsPerSecond
			}
			tm.mutex.RUnlock()
			
			// Also observe total EPS
			observer.Observe(totalEPS, metric.WithAttributes(
				attribute.String("stream_name", "total"),
				))
				
				return nil
			}
			
			// NewMetricsCollector creates a new metrics collector
			func NewMetricsCollector(telemetry *TelemetryManager) *MetricsCollector {
				return &MetricsCollector{
					telemetry:  telemetry,
					streams:    make(map[string]models.Stream),
					collectors: make(map[string]*StreamCollector),
					stopChan:   make(chan struct{}),
				}
			}
			
			// Start starts the metrics collector
			func (mc *MetricsCollector) Start(ctx context.Context, interval time.Duration) error {
				mc.mutex.Lock()
				defer mc.mutex.Unlock()
				
				if mc.ticker != nil {
					return fmt.Errorf("metrics collector already started")
				}
				
				mc.ticker = time.NewTicker(interval)
				
				go mc.collectLoop(ctx)
				
				log.Info().
				Dur("interval", interval).
				Msg("Metrics collector started")
				
				return nil
			}
			
			// Stop stops the metrics collector
			func (mc *MetricsCollector) Stop() error {
				mc.mutex.Lock()
				defer mc.mutex.Unlock()
				
				if mc.ticker == nil {
					return nil
				}
				
				mc.ticker.Stop()
				close(mc.stopChan)
				mc.ticker = nil
				
				log.Info().Msg("Metrics collector stopped")
				return nil
			}
			
			// AddStream adds a stream to be monitored
			func (mc *MetricsCollector) AddStream(stream models.Stream) {
				mc.mutex.Lock()
				defer mc.mutex.Unlock()
				
				streamName := stream.GetConfig().Name
				mc.streams[streamName] = stream
				mc.collectors[streamName] = &StreamCollector{
					streamName: streamName,
					stream:     stream,
					telemetry:  mc.telemetry,
				}
				
				log.Info().
				Str("stream_name", streamName).
				Msg("Added stream to metrics collection")
			}
			
			// RemoveStream removes a stream from monitoring
			func (mc *MetricsCollector) RemoveStream(streamName string) {
				mc.mutex.Lock()
				defer mc.mutex.Unlock()
				
				delete(mc.streams, streamName)
				delete(mc.collectors, streamName)
				
				log.Info().
				Str("stream_name", streamName).
				Msg("Removed stream from metrics collection")
			}
			
			// collectLoop runs the metrics collection loop
			func (mc *MetricsCollector) collectLoop(ctx context.Context) {
				for {
					select {
					case <-mc.ticker.C:
						mc.collectMetrics(ctx)
					case <-mc.stopChan:
						return
					case <-ctx.Done():
						return
					}
				}
			}
			
			// collectMetrics collects metrics from all streams
			func (mc *MetricsCollector) collectMetrics(ctx context.Context) {
				mc.mutex.RLock()
				collectors := make(map[string]*StreamCollector)
				for name, collector := range mc.collectors {
					collectors[name] = collector
				}
				mc.mutex.RUnlock()
				
				for _, collector := range collectors {
					collector.collectMetrics(ctx)
				}
			}
			
			// collectMetrics collects metrics for a specific stream
			func (sc *StreamCollector) collectMetrics(ctx context.Context) {
				sc.mutex.Lock()
				defer sc.mutex.Unlock()
				
				metrics := sc.stream.GetMetrics()
				sc.telemetry.UpdateStreamMetrics(sc.streamName, metrics)
				sc.lastMetrics = metrics
			}
			
// DefaultTelemetryConfig returns a default telemetry configuration
func DefaultTelemetryConfig() TelemetryConfig {
	return config.DefaultConfig().Telemetry
}
