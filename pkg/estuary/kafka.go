package estuary

import (
	"fmt"
	"log"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/Shopify/sarama"
	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
)

/*
Simple SyncProducer for kafka

Taken from: https://github.com/Shopify/sarama/blob/master/examples/http_server/http_server.go
If this works ok for mock - then add snappy...

*/

type KafkaEndpoint struct {
	producer sarama.SyncProducer
	topic    string
}

func (s KafkaEndpoint) WriteEvent(record *events.RecordEvent) {
	// We are not setting a message key, which means that all messages will
	// be distributed randomly over the different partitions.

	data, err := ffjson.Marshal(record)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to send event... :( ")
	}

	partition, offset, err := s.producer.SendMessage(&sarama.ProducerMessage{
		Topic: s.topic,
		Value: sarama.ByteEncoder(data),
	})

	if err != nil {
		logger.Error().Err(err).Msg("Failed to send event... :( ")
	} else {
		// The tuple (topic, partition, offset) can be used as a unique identifier
		// for a message in a Kafka cluster.
		logger.Debug().Msgf("Your data is stored with unique identifier important/%d/%d", partition, offset)
	}
	logger.Debug().Msgf("record: %v", record)
}

func NewKafkaEndpoint(streamConfig *config.WaterFlowsConfig) (endpoint KafkaEndpoint) {
	brokers := []string{fmt.Sprintf("%s:%d", streamConfig.Host, streamConfig.Port)}
	producer := newDataCollector(brokers)
	endpoint = KafkaEndpoint{
		producer: producer,
		topic:    streamConfig.Schema,
	}
	return endpoint
}

func newDataCollector(brokerList []string) sarama.SyncProducer {

	// For the data collector, we are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	config.Producer.Return.Successes = true

	// tlsConfig := createTlsConfiguration()
	// if tlsConfig != nil {
	// 	config.Net.TLS.Config = tlsConfig
	// 	config.Net.TLS.Enable = true
	// }

	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.

	producer, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		log.Fatalln("Failed to start Sarama producer:", err)
	}

	return producer
}

func (s *KafkaEndpoint) Close() error {
	if err := s.producer.Close(); err != nil {
		logger.Error().Err(err).Msg("Failed to shut down data collector cleanly")
	}

	return nil
}
