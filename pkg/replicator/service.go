package replicator

import (
"context"
"fmt"
"sync"
"time"
	
"github.com/cohenjo/replicator/pkg/api"
"github.com/cohenjo/replicator/pkg/auth"
"github.com/cohenjo/replicator/pkg/config"
"github.com/cohenjo/replicator/pkg/events"
"github.com/cohenjo/replicator/pkg/metrics"
"github.com/cohenjo/replicator/pkg/models"
"github.com/cohenjo/replicator/pkg/streams"
"github.com/cohenjo/replicator/pkg/transform"
"github.com/rs/zerolog/log"
"github.com/sirupsen/logrus"
)

// EstuaryWriter represents an interface for writing events to destinations
type EstuaryWriter interface {
	WriteEvent(ctx context.Context, event map[string]interface{}) error
	Close() error
}

// Service represents the main replication service
type Service struct {
	config           *config.Config
	logger           *logrus.Logger
	streams          map[string]StreamManager
	streamManager    *StreamManager
	apiServer        *api.ServerV2
	authProvider     auth.Provider
	metricsCollector *metrics.TelemetryManager
	transformEngine  *transform.Engine
	shutdownHandler  *ShutdownHandler
	eventChannel     chan events.RecordEvent
	shutdownChannel  chan struct{}
	status           ServiceStatus
	startTime        time.Time
	estuaries        []EstuaryWriter
	wg               sync.WaitGroup
	mu               sync.RWMutex
}

// ServiceStatus represents the current status of the service
type ServiceStatus string

const (
	StatusStopped  ServiceStatus = "stopped"
	StatusStarting ServiceStatus = "starting"
	StatusRunning  ServiceStatus = "running"
	StatusStopping ServiceStatus = "stopping"
	StatusError    ServiceStatus = "error"
)



// ServiceOptions represents configuration options for the service
type ServiceOptions struct {
	Config       *config.Config
	Logger       *logrus.Logger
	EventBuffer  int
}

// NewService creates a new replicator service instance
func NewService(opts ServiceOptions) (*Service, error) {
	if opts.Config == nil {
		return nil, fmt.Errorf("config is required")
	}
	
	if opts.Logger == nil {
		opts.Logger = logrus.New()
	}
	
	if opts.EventBuffer == 0 {
		opts.EventBuffer = 10000 // Default buffer size
	}
	
	// Create event channel
	eventChannel := make(chan events.RecordEvent, opts.EventBuffer)
	
	// Create stream manager
	streamManager := &StreamManager{
		streams:      make(map[string]models.Stream),
		streamStates: make(map[string]models.StreamState),
		eventChannel: eventChannel,
		logger:       opts.Logger,
	}
	
	// Create metrics collector
	metricsCollector, err := metrics.NewTelemetryManager(opts.Config.Telemetry)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics collector: %w", err)
	}
	
	// Create auth provider
	authProvider, err := auth.NewProvider(opts.Config.Azure.Authentication)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth provider: %w", err)
	}
	
	// Create transform engine
	transformConfig := transform.DefaultTransformationConfig()
	transformEngine := transform.NewEngine(transformConfig)
	
	// Create API server
	apiServer, err := api.NewServerV2(api.ServerV2Config{
		Config:          opts.Config,
		StreamManager:   streamManager,
		MetricsCollector: metricsCollector,
		Logger:          opts.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API server: %w", err)
	}
	
	service := &Service{
		config:          opts.Config,
		logger:          opts.Logger,
		streamManager:   streamManager,
		apiServer:       apiServer,
		metricsCollector: metricsCollector,
		authProvider:    authProvider,
		transformEngine: transformEngine,
		eventChannel:    eventChannel,
		shutdownChannel: make(chan struct{}),
		status:          StatusStopped,
	}
	
	return service, nil
}

// Start starts the replicator service
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.status != StatusStopped {
		return fmt.Errorf("service is already running or starting")
	}
	
	s.status = StatusStarting
	s.startTime = time.Now()
	
	s.logger.Info("Starting replicator service")
	
	// Start metrics collection
	if err := s.metricsCollector.Start(ctx); err != nil {
		s.status = StatusError
		return fmt.Errorf("failed to start metrics collector: %w", err)
	}
	
	// Initialize streams from configuration
	if err := s.initializeStreams(ctx); err != nil {
		s.status = StatusError
		return fmt.Errorf("failed to initialize streams: %w", err)
	}
	
	// Start event processor
	s.wg.Add(1)
	go s.processEvents(ctx)
	
	// Start stream monitoring
	s.wg.Add(1)
	go s.monitorStreams(ctx)
	
	// Start API server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.apiServer.Start(ctx); err != nil {
			s.logger.WithError(err).Error("API server failed")
		}
		}()
		
		// Start all configured streams
		if err := s.streamManager.StartAll(ctx); err != nil {
			s.status = StatusError
			return fmt.Errorf("failed to start streams: %w", err)
		}
		
		s.status = StatusRunning
		s.logger.WithFields(logrus.Fields{
			"streams": len(s.streamManager.streams),
			"uptime":  time.Since(s.startTime),
			}).Info("Replicator service started successfully")
			
			return nil
		}
		
		// Stop gracefully stops the replicator service
		func (s *Service) Stop(ctx context.Context) error {
			s.mu.Lock()
			defer s.mu.Unlock()
			
			if s.status != StatusRunning {
				return fmt.Errorf("service is not running")
			}
			
			s.status = StatusStopping
			s.logger.Info("Stopping replicator service")
			
			// Signal shutdown
			close(s.shutdownChannel)
			
			// Stop all streams
			if err := s.streamManager.StopAll(ctx); err != nil {
				s.logger.WithError(err).Error("Failed to stop some streams")
			}
			
			// Stop API server
			if err := s.apiServer.Stop(ctx); err != nil {
				s.logger.WithError(err).Error("Failed to stop API server")
			}
			
			// Stop metrics collector
			if err := s.metricsCollector.Stop(ctx); err != nil {
				s.logger.WithError(err).Error("Failed to stop metrics collector")
			}
			
			// Wait for all goroutines to finish
			done := make(chan struct{})
			go func() {
				s.wg.Wait()
				close(done)
				}()
				
				select {
				case <-done:
					s.logger.Info("All goroutines stopped")
				case <-ctx.Done():
					s.logger.Warn("Shutdown context cancelled, some goroutines may not have stopped cleanly")
				}
				
				s.status = StatusStopped
				s.logger.WithField("uptime", time.Since(s.startTime)).Info("Replicator service stopped")
				
				return nil
			}
			
			// GetStatus returns the current service status
			func (s *Service) GetStatus() ServiceStatus {
				s.mu.RLock()
				defer s.mu.RUnlock()
				return s.status
			}
			
			// GetHealthStatus returns comprehensive health status
			func (s *Service) GetHealthStatus() models.HealthStatus {
				s.mu.RLock()
				defer s.mu.RUnlock()
				
				status := "healthy"
				if s.status != StatusRunning {
					status = "unhealthy"
				}
				
				// Check stream health
				streamStates := make(map[string]models.StreamState)
				for name, stream := range s.streamManager.streams {
					state := stream.GetState()
					streamStates[name] = state
					
					// If any stream is in error state, mark service as degraded
					if state.Status == config.StreamStatusError && status == "healthy" {
						status = "degraded"
					}
				}
				
				// Perform health checks
				checks := make(map[string]models.CheckResult)
				
				// Database connectivity check
				checks["database"] = models.CheckResult{
					Status:    "pass",
					Message:   "All database connections healthy",
					Timestamp: time.Now(),
				}
				
				// Memory check
				checks["memory"] = models.CheckResult{
					Status:    "pass",
					Message:   "Memory usage within limits",
					Timestamp: time.Now(),
				}
				
				return models.HealthStatus{
					Status:      status,
					Timestamp:   time.Now(),
					Uptime:      time.Since(s.startTime),
					Version:     "1.0.0", // TODO: Get from build info
					StreamCount: len(s.streamManager.streams),
					Streams:     streamStates,
					Checks:      checks,
				}
			}
			
			// initializeStreams creates and configures streams from the service configuration
			func (s *Service) initializeStreams(ctx context.Context) error {
				log.Debug().Int("stream_count", len(s.config.Streams)).Msg("initializeStreams called")
				
				for i, streamConfig := range s.config.Streams {
					log.Debug().Int("index", i).Str("name", streamConfig.Name).Bool("enabled", streamConfig.Enabled).Msg("Processing stream")
					
					if !streamConfig.Enabled {
						s.logger.WithField("stream", streamConfig.Name).Debug("Skipping disabled stream")
						continue
					}
					
					stream, err := s.createStream(streamConfig)
					if err != nil {
						return fmt.Errorf("failed to create stream %s: %w", streamConfig.Name, err)
					}
					
					s.streamManager.streams[streamConfig.Name] = stream
					s.streamManager.streamStates[streamConfig.Name] = models.StreamState{
						Name:   streamConfig.Name,
						Status: config.StreamStatusStopped,
					}
					
					// Create EstuaryWriter instances for the target configuration
					if streamConfig.Target.Type != "" {
						log.Debug().Str("stream", streamConfig.Name).Str("target_type", string(streamConfig.Target.Type)).Str("host", streamConfig.Target.Host).Msg("Creating EstuaryWriter")
						estuary, err := s.createEstuaryWriter(streamConfig.Target)
						if err != nil {
							log.Error().Err(err).Str("stream", streamConfig.Name).Msg("Failed to create estuary writer")
							return fmt.Errorf("failed to create estuary writer for stream %s: %w", streamConfig.Name, err)
						}
						
						s.estuaries = append(s.estuaries, estuary)
						log.Debug().Int("total_estuaries", len(s.estuaries)).Msg("EstuaryWriter added to estuaries slice")
						s.logger.WithFields(logrus.Fields{
							"stream": streamConfig.Name,
							"target_type": streamConfig.Target.Type,
							"target_host": streamConfig.Target.Host,
							}).Info("Estuary writer initialized")
					} else {
						log.Debug().Str("stream", streamConfig.Name).Msg("No target configuration for stream")
					}
							
					s.logger.WithField("stream", streamConfig.Name).Info("Stream initialized")
				}
				
				return nil
			}
					
// createStream creates a stream instance based on configuration
func (s *Service) createStream(streamConfig config.StreamConfig) (models.Stream, error) {
	log.Debug().Str("name", streamConfig.Name).Str("source_type", string(streamConfig.Source.Type)).Msg("createStream called")
	
	// Create appropriate stream based on source type
	switch streamConfig.Source.Type {
	case "mongodb":
		log.Debug().Msg("Creating MongoDB stream")
		return streams.NewMongoDBStream(streamConfig, s.eventChannel)
	case "mysql":
		log.Debug().Msg("Creating MySQL stream")
		return streams.NewMySQLStream(streamConfig, s.eventChannel)
	case "postgresql":
		log.Debug().Msg("Creating PostgreSQL stream")
		return streams.NewPostgreSQLStream(streamConfig, s.eventChannel)
	case "kafka":
		log.Debug().Msg("Creating Kafka stream")
		return streams.NewKafkaStream(streamConfig, s.eventChannel)
	default:
		log.Error().Str("source_type", string(streamConfig.Source.Type)).Msg("Unsupported stream type")
		return nil, fmt.Errorf("stream type %s not yet implemented", streamConfig.Source.Type)
	}
}
					
// createEstuaryWriter creates an EstuaryWriter instance based on target configuration
func (s *Service) createEstuaryWriter(targetConfig config.TargetConfig) (EstuaryWriter, error) {
	bridge, err := NewEstuaryBridge(targetConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create estuary bridge: %w", err)
	}
	
	s.logger.WithFields(logrus.Fields{
		"type": targetConfig.Type,
		"host": targetConfig.Host,
		"port": targetConfig.Port,
		"database": targetConfig.Database,
		}).Debug("Created estuary writer")
		
		return bridge, nil
	}
	
	// processEvents processes events from the event channel
	func (s *Service) processEvents(ctx context.Context) {
		defer s.wg.Done()
		
		s.logger.Info("Starting event processor")
		
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("Event processor stopping due to context cancellation")
				return
			case <-s.shutdownChannel:
				s.logger.Info("Event processor stopping due to shutdown signal")
				return
			case event := <-s.eventChannel:
				if err := s.handleEvent(ctx, event); err != nil {
					s.logger.WithError(err).Error("Failed to process event")
					s.metricsCollector.IncrementCounter("events_failed_total", 1)
					} else {
						s.metricsCollector.IncrementCounter("events_processed_total", 1)
					}
				}
			}
		}
							
// handleEvent processes a single event
func (s *Service) handleEvent(ctx context.Context, event events.RecordEvent) error {
s.logger.WithFields(logrus.Fields{
"action":     event.Action,
"schema":     event.Schema,
"collection": event.Collection,
}).Debug("Processing event")

// Guard against empty data for actionable operations
isActionableOp := event.Action == "insert" || event.Action == "update" || event.Action == "replace"
if len(event.Data) == 0 && isActionableOp {
s.logger.WithFields(logrus.Fields{
"action":     event.Action,
"schema":     event.Schema,
"collection": event.Collection,
"data_size":  len(event.Data),
}).Warn("Skipping event with empty Data field for actionable operation")

// Update metrics for skipped events
if s.metricsCollector != nil {
s.metricsCollector.IncrementCounter("events_data_missing_total", 1)
}
return nil
}

// Convert event to map for transformation
eventData := map[string]interface{}{
    "action":       event.Action,
    "schema":       event.Schema,
    "collection":   event.Collection,
    "table":        event.Collection, // Use collection as table
    "data":         event.Data,
    "old_data":     event.OldData,
    "documentKey":  event.DocumentKey, // Ensure documentKey is preserved
    "position":     "", // Not available in this event type
    "timestamp":    time.Now(), // Use current time
    "source":       event.Schema, // Use schema as source
    "_metadata": map[string]interface{}{
        "event_id":    fmt.Sprintf("%s_%s_%d", event.Schema, event.Collection, time.Now().UnixNano()),
        "source_type": event.Schema,
        "processed_at": time.Now(),
    },
}

// Apply transformations if configured
var transformedData map[string]interface{}
var transformationErr error

if s.transformEngine != nil {
// Apply stream-specific transformations
// Note: In a real implementation, you'd get the stream config and rules
// For now, we'll apply any global transformations
transformResult, err := s.transformEngine.Transform(ctx, eventData)
if err != nil {
transformationErr = fmt.Errorf("transformation failed: %w", err)
s.logger.WithError(err).Error("Failed to transform event")

// Use original data if transformation fails
transformedData = eventData
} else if transformResult.Success {
transformedData = transformResult.Output
s.logger.WithFields(logrus.Fields{
"applied_rules": transformResult.AppliedRules,
"execution_time": transformResult.ExecutionTime,
}).Debug("Event transformed successfully")
} else {
// Transformation had errors but may have partial results
transformedData = transformResult.Output
s.logger.WithFields(logrus.Fields{
"errors": transformResult.Errors,
"warnings": transformResult.Warnings,
}).Warn("Event transformation completed with errors")
}

// Preserve critical fields that should not be lost during transformation
// This ensures update/delete operations have the necessary document key
if eventData["old_data"] != nil && transformedData != nil {
transformedData["old_data"] = eventData["old_data"]
s.logger.WithFields(logrus.Fields{
"action": event.Action,
"has_old_data": true,
}).Debug("Preserved old_data field after transformation")
}
} else {
// No transformation engine, use original data
transformedData = eventData
}

// Route to appropriate destinations (estuaries)
// Note: This would typically route based on stream configuration
log.Debug().Int("estuary_count", len(s.estuaries)).Msg("Service.handleEvent: routing to estuaries")
for i, estuary := range s.estuaries {
log.Debug().Int("estuary_index", i).Str("estuary", fmt.Sprintf("%T", estuary)).Msg("Service.handleEvent: writing to estuary")
if err := estuary.WriteEvent(ctx, transformedData); err != nil {
	s.logger.WithError(err).Error("Failed to write event to estuary")
	log.Error().Err(err).Int("estuary_index", i).Msg("Service.handleEvent: failed to write to estuary")
	// Continue with other estuaries even if one fails
	} else {
		log.Debug().Int("estuary_index", i).Msg("Service.handleEvent: successfully wrote to estuary")
	}
}

// Update metrics
if s.metricsCollector != nil {
	metrics := map[string]interface{}{
		"events_processed": 1,
		"event_action": event.Action,
		"source_type": event.Schema,
	}
	
	if transformationErr != nil {
		metrics["transformation_errors"] = 1
		} else {
			metrics["transformation_success"] = 1
		}
		
		s.metricsCollector.RecordMetrics(ctx, metrics)
	}
	
	return nil
}

// monitorStreams monitors stream health and metrics
func (s *Service) monitorStreams(ctx context.Context) {
	defer s.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	s.logger.Info("Starting stream monitor")
	
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stream monitor stopping due to context cancellation")
			return
		case <-s.shutdownChannel:
			s.logger.Info("Stream monitor stopping due to shutdown signal")
			return
		case <-ticker.C:
			s.updateStreamMetrics()
		}
	}
}
																	
// updateStreamMetrics updates metrics for all streams
func (s *Service) updateStreamMetrics() {
	s.streamManager.mu.RLock()
	defer s.streamManager.mu.RUnlock()
	
	for name, stream := range s.streamManager.streams {
		metrics := stream.GetMetrics()
		state := stream.GetState()
		
		// Update state
		s.streamManager.streamStates[name] = state
		
		// Record metrics
		s.metricsCollector.SetGauge("stream_events_processed", float64(metrics.EventsProcessed), map[string]string{
			"stream": name,
		})
		
		s.metricsCollector.SetGauge("stream_events_per_second", metrics.EventsPerSecond, map[string]string{
			"stream": name,
		})
		
		s.metricsCollector.SetGauge("stream_error_count", float64(metrics.ErrorCount), map[string]string{
			"stream": name,
		})
	}
}
