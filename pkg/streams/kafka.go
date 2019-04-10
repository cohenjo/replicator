package streams

import (
	"context"
	"os"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/Shopify/sarama"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/rs/zerolog"
)

/*
This is a basic consumer group based of the example in: https://github.com/Shopify/sarama/tree/master/examples/consumergroup
messages in the stream are just `ffjson.Marshel` events, we only unmarshel and add to the event channel.
*/

type KafkaStream struct {
	events     *chan *events.RecordEvent
	topic      string
	collection string
	client     *sarama.ConsumerGroup
}

func NewKafkaStream(events *chan *events.RecordEvent, schema string, collection string) (stream KafkaStream) {
	stream.events = events
	stream.topic = schema
	stream.collection = collection
	return stream
}

func (stream KafkaStream) StreamType() string {
	return "Kafka"
}

func (stream KafkaStream) Listen() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	// version, err := sarama.ParseKafkaVersion("2.1.1")
	// if err != nil {
	// 	logger.Error().Err(err).Msg("Failed to parse version")
	// 	panic(err)
	// }
	/**
	 * Construct a new Sarama configuration.
	 * The Kafka cluster version has to be defined before the consumer/producer is initialized.
	 */
	config := sarama.NewConfig()
	// config.Version = version
	config.Version = sarama.V1_0_0_0
	config.Consumer.Return.Errors = true

	oldest := true
	if oldest {
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	/**
	 * Setup a new Sarama consumer group
	 */
	consumer := Consumer{
		ready:  make(chan bool, 0),
		events: stream.events,
	}

	ctx := context.Background()
	// brokers := []string{"localhost:9092"}
	topics := []string{"db-replicator"}
	// group := "example"
	client, err := sarama.NewClient([]string{"localhost:9092"}, config)
	if err != nil {
		panic(err)
	}
	defer func() { _ = client.Close() }()

	// Start a new consumer group
	group, err := sarama.NewConsumerGroupFromClient("my-group", client)
	if err != nil {
		panic(err)
	}
	defer func() { _ = group.Close() }()
	// client, err := sarama.NewConsumerGroup(brokers, group, config)
	if err != nil {
		panic(err)
	}
	stream.client = &group

	// Track errors
	go func() {
		for err := range group.Errors() {
			logger.Error().Err(err).Msg("error")

		}
	}()

	for {
		logger.Info().Msg("consume stuff")
		err := group.Consume(ctx, topics, &consumer)
		if err != nil {
			panic(err)
		}
	}
	<-consumer.ready // Await till the consumer has been set up
	logger.Info().Msg("Sarama consumer up and running!...")

}

// Consumer represents a Sarama consumer group consumer
type Consumer struct {
	ready  chan bool
	events *chan *events.RecordEvent
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *Consumer) Setup(sarama.ConsumerGroupSession) error {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger.Info().Msg("in Setup")
	// Mark the consumer as ready
	close(consumer.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger.Info().Msg("in Cleanup")
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger.Info().Msg("in ConsumeClaim")
	// NOTE:
	// Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/master/consumer_group.go#L27-L29
	for message := range claim.Messages() {
		var record events.RecordEvent
		err := ffjson.Unmarshal(message.Value, &record)
		if err != nil {
			logger.Error().Err(err).Msgf("guess it wasn't an event")
			continue
		}
		logger.Info().Msgf("Message claimed: value = %s, timestamp = %v, topic = %s", record.Action, message.Timestamp, message.Topic)
		session.MarkMessage(message, "")
		if consumer.events != nil {
			*consumer.events <- &record
		}
	}

	return nil
}

// func (s KafkaStream) Quit() {
// 	err := *s.client.Close()
// 	if err != nil {
// 		panic(err)
// 	}
// }
