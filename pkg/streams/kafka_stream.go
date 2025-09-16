package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog/log"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/cohenjo/replicator/pkg/models"
)

// KafkaStream implements the models.Stream interface for Kafka consumption
type KafkaStream struct {
	config        config.StreamConfig
	consumer      sarama.ConsumerGroup
	state         models.StreamState
	metrics       models.ReplicationMetrics
	eventChannel  chan<- events.RecordEvent
	stopChan      chan struct{}
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	consumerGroup string
	topics        []string
}

// NewKafkaStream creates a new Kafka stream instance
func NewKafkaStream(streamConfig config.StreamConfig, eventChannel chan<- events.RecordEvent) (*KafkaStream, error) {
	// Validate configuration
	if streamConfig.Source.Type != "kafka" {
		return nil, fmt.Errorf("invalid source type for Kafka stream: %s", streamConfig.Source.Type)
	}

	// Extract consumer group and topics from config
	consumerGroup := "replicator-group"
	if streamConfig.Source.Options != nil {
		if group, ok := streamConfig.Source.Options["consumer_group"].(string); ok && group != "" {
			consumerGroup = group
		}
	}

	topics := []string{"events"}
	if streamConfig.Source.Options != nil {
		if topicList, ok := streamConfig.Source.Options["topics"].([]interface{}); ok {
			topics = make([]string, len(topicList))
			for i, topic := range topicList {
				if topicStr, ok := topic.(string); ok {
					topics[i] = topicStr
				}
			}
		} else if topicStr, ok := streamConfig.Source.Options["topics"].(string); ok {
			topics = strings.Split(topicStr, ",")
			for i := range topics {
				topics[i] = strings.TrimSpace(topics[i])
			}
		}
	}

	return &KafkaStream{
		config:        streamConfig,
		eventChannel:  eventChannel,
		stopChan:      make(chan struct{}),
		consumerGroup: consumerGroup,
		topics:        topics,
		state: models.StreamState{
			Name:   streamConfig.Name,
			Status: config.StreamStatusStopped,
		},
		metrics: models.ReplicationMetrics{
			StreamName: streamConfig.Name,
		},
	}, nil
}

// Start begins the Kafka consumption stream
func (s *KafkaStream) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status == config.StreamStatusRunning {
		return fmt.Errorf("stream is already running")
	}

	log.Info().Str("stream", s.config.Name).Msg("Starting Kafka stream")

	// Create context for this stream
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Setup Kafka consumer
	if err := s.setupConsumer(); err != nil {
		s.state.Status = config.StreamStatusError
		lastError := err.Error()
		s.state.LastError = &lastError
		return fmt.Errorf("failed to setup Kafka consumer: %w", err)
	}

	// Update state
	s.state.Status = config.StreamStatusRunning
	now := time.Now()
	s.state.StartedAt = &now
	s.state.LastError = nil
	s.metrics.LastProcessedTime = time.Now()

	// Start consuming in background
	go s.consume()

	log.Info().Str("stream", s.config.Name).Msg("Kafka stream started successfully")
	return nil
}

// Stop gracefully stops the Kafka consumption stream
func (s *KafkaStream) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status == config.StreamStatusStopped {
		return nil
	}

	log.Info().Str("stream", s.config.Name).Msg("Stopping Kafka stream")

	// Cancel context to stop processing
	if s.cancel != nil {
		s.cancel()
	}

	// Close consumer
	if s.consumer != nil {
		if err := s.consumer.Close(); err != nil {
			log.Error().Err(err).Str("stream", s.config.Name).Msg("Failed to close Kafka consumer")
		}
	}

	// Update state
	s.state.Status = config.StreamStatusStopped
	now := time.Now()
	s.state.StoppedAt = &now

	log.Info().Str("stream", s.config.Name).Msg("Kafka stream stopped")
	return nil
}

// Pause temporarily pauses the Kafka consumption stream
func (s *KafkaStream) Pause(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status != config.StreamStatusRunning {
		return fmt.Errorf("stream is not running")
	}

	s.state.Status = config.StreamStatusPaused
	log.Info().Str("stream", s.config.Name).Msg("Kafka stream paused")
	return nil
}

// Resume resumes a paused Kafka consumption stream
func (s *KafkaStream) Resume(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Status != config.StreamStatusPaused {
		return fmt.Errorf("stream is not paused")
	}

	s.state.Status = config.StreamStatusRunning
	log.Info().Str("stream", s.config.Name).Msg("Kafka stream resumed")
	return nil
}

// GetState returns the current state of the stream
func (s *KafkaStream) GetState() models.StreamState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// GetConfig returns the configuration of the stream
func (s *KafkaStream) GetConfig() config.StreamConfig {
	return s.config
}

// GetMetrics returns current metrics for the stream
func (s *KafkaStream) GetMetrics() models.ReplicationMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Calculate events per second
	if s.metrics.EventsProcessed > 0 && !s.metrics.LastProcessedTime.IsZero() {
		duration := time.Since(s.metrics.LastProcessedTime)
		if duration > 0 {
			s.metrics.EventsPerSecond = float64(s.metrics.EventsProcessed) / duration.Seconds()
		}
	}

	return s.metrics
}

// SetCheckpoint updates the stream checkpoint
func (s *KafkaStream) SetCheckpoint(checkpoint map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store checkpoint (Kafka manages offsets automatically in consumer groups)
	log.Debug().Interface("checkpoint", checkpoint).Str("stream", s.config.Name).Msg("Checkpoint updated")
	return nil
}

// GetCheckpoint returns the current checkpoint
func (s *KafkaStream) GetCheckpoint() (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return current checkpoint (Kafka consumer group manages offsets)
	return make(map[string]interface{}), nil
}

// setupConsumer configures the Kafka consumer
func (s *KafkaStream) setupConsumer() error {
	// Build Kafka configuration
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true
	config.Version = sarama.V2_6_0_0

	// Enable auto-commit by default
	config.Consumer.Group.Session.Timeout = 10 * time.Second
	config.Consumer.Group.Heartbeat.Interval = 3 * time.Second

	// Configure authentication if provided
	if s.config.Source.Username != "" && s.config.Source.Password != "" {
		config.Net.SASL.Enable = true
		config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		config.Net.SASL.User = s.config.Source.Username
		config.Net.SASL.Password = s.config.Source.Password
	}

	// Configure TLS if needed
	if s.config.Source.Options != nil {
		if useTLS, ok := s.config.Source.Options["use_tls"].(bool); ok && useTLS {
			config.Net.TLS.Enable = true
		}
	}

	// Build broker list
	brokers := []string{fmt.Sprintf("%s:%d", s.config.Source.Host, s.config.Source.Port)}
	if s.config.Source.Options != nil {
		if brokerList, ok := s.config.Source.Options["brokers"].([]interface{}); ok {
			brokers = make([]string, len(brokerList))
			for i, broker := range brokerList {
				if brokerStr, ok := broker.(string); ok {
					brokers[i] = brokerStr
				}
			}
		}
	}

	// Create consumer group
	consumer, err := sarama.NewConsumerGroup(brokers, s.consumerGroup, config)
	if err != nil {
		return fmt.Errorf("failed to create Kafka consumer group: %w", err)
	}

	s.consumer = consumer
	return nil
}

// consume starts the Kafka consumption process
func (s *KafkaStream) consume() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("stream", s.config.Name).Msg("Panic in Kafka consumption")
			s.mu.Lock()
			s.state.Status = config.StreamStatusError
			lastError := fmt.Sprintf("panic: %v", r)
			s.state.LastError = &lastError
			s.mu.Unlock()
		}
	}()

	log.Info().Str("stream", s.config.Name).Strs("topics", s.topics).Msg("Starting Kafka consumption")

	// Create consumer group handler
	handler := &consumerGroupHandler{
		stream: s,
	}

	for {
		select {
		case <-s.ctx.Done():
			log.Info().Str("stream", s.config.Name).Msg("Kafka consumption stopped")
			return
		default:
			// Consume from topics
			if err := s.consumer.Consume(s.ctx, s.topics, handler); err != nil {
				log.Error().Err(err).Str("stream", s.config.Name).Msg("Kafka consumption error")
				s.mu.Lock()
				s.metrics.ErrorCount++
				s.mu.Unlock()
				
				// If there's an error, wait a bit before retrying
				select {
				case <-s.ctx.Done():
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}
		}
	}
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	stream *KafkaStream
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	log.Debug().Str("stream", h.stream.config.Name).Msg("Kafka consumer session setup")
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	log.Debug().Str("stream", h.stream.config.Name).Msg("Kafka consumer session cleanup")
	return nil
}

// ConsumeClaim processes messages from a topic/partition claim
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// Process messages
	for {
		select {
		case <-h.stream.ctx.Done():
			return nil
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			// Check if stream is paused
			h.stream.mu.RLock()
			isPaused := h.stream.state.Status == config.StreamStatusPaused
			h.stream.mu.RUnlock()

			if isPaused {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Process the message
			if err := h.processMessage(message); err != nil {
				log.Error().Err(err).Str("stream", h.stream.config.Name).Msg("Failed to process Kafka message")
				h.stream.mu.Lock()
				h.stream.metrics.ErrorCount++
				h.stream.mu.Unlock()
				continue
			}

			// Mark message as processed
			session.MarkMessage(message, "")

			// Update metrics
			h.stream.mu.Lock()
			h.stream.metrics.EventsProcessed++
			h.stream.metrics.LastProcessedTime = time.Now()
			h.stream.mu.Unlock()
		}
	}
}

// processMessage processes a single Kafka message
func (h *consumerGroupHandler) processMessage(message *sarama.ConsumerMessage) error {
	// Try to parse the message as JSON to extract action and other metadata
	var messageData map[string]interface{}
	if err := json.Unmarshal(message.Value, &messageData); err != nil {
		// If we can't parse as JSON, treat the whole message as data
		messageData = map[string]interface{}{
			"value": string(message.Value),
		}
	}

	// Extract action if available, default to "insert"
	action := "insert"
	if actionValue, ok := messageData["action"].(string); ok {
		action = actionValue
	}

	// Extract schema/database name
	schema := h.stream.config.Source.Database
	if schemaValue, ok := messageData["schema"].(string); ok {
		schema = schemaValue
	}

	// Extract collection/topic name (use Kafka topic as collection)
	collection := message.Topic
	if collectionValue, ok := messageData["collection"].(string); ok {
		collection = collectionValue
	}

	// Marshal the complete message data
	data, err := json.Marshal(messageData)
	if err != nil {
		return fmt.Errorf("failed to marshal message data: %w", err)
	}

	// Create replication event
	recordEvent := events.RecordEvent{
		Action:     action,
		Schema:     schema,
		Collection: collection,
		Data:       data,
	}

	// Add Kafka-specific metadata
	if len(message.Headers) > 0 {
		headers := make(map[string]string)
		for _, header := range message.Headers {
			headers[string(header.Key)] = string(header.Value)
		}
		// Could store headers in the event if needed
	}

	// Send to event channel (non-blocking)
	select {
	case h.stream.eventChannel <- recordEvent:
		log.Debug().
			Str("stream", h.stream.config.Name).
			Str("topic", message.Topic).
			Int32("partition", message.Partition).
			Int64("offset", message.Offset).
			Str("action", action).
			Msg("Kafka message sent to processing pipeline")
	default:
		log.Warn().
			Str("stream", h.stream.config.Name).
			Msg("Event channel full, dropping Kafka message")
		h.stream.mu.Lock()
		h.stream.metrics.ErrorCount++
		h.stream.mu.Unlock()
	}

	return nil
}