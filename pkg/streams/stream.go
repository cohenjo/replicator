package streams

import (
	"os"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
)

type Stream interface {
	// Configure(events *chan *events.RecordEvent, schema string, collection string)
	Listen()
	StreamType() string
}

type StreamManagment struct {
	events  *chan *events.RecordEvent
	streams []Stream
	quit    chan bool
}

var (
	recordsRecieved = promauto.NewCounter(prometheus.CounterOpts{
		Name: "replicator_recieved_records_total",
		Help: "The total number of records recieved",
	})
	StreamManager *StreamManagment
	logger        = zerolog.New(os.Stderr).With().Timestamp().Logger()
)

func SetupStreamManager(events *chan *events.RecordEvent) {
	StreamManager = &StreamManagment{
		events:  events,
		streams: make([]Stream, 0),
		quit:    make(chan bool),
	}
}

func (sm *StreamManagment) NewStream(streamConfig *config.WaterFlowsConfig) {
	var stream Stream
	switch streamConfig.Type {
	case "MYSQL":
		stream = NewMySQLStream(sm.events, streamConfig)
	case "MONGO":
		stream = NewMongoStream(sm.events, streamConfig)
	case "KAFKA":
		stream = NewKafkaStream(sm.events, streamConfig)
	}
	sm.streams = append(sm.streams, stream)
}

func (sm *StreamManagment) registerStreams(stream Stream) {
	sm.streams = append(sm.streams, stream)
}

func (em *StreamManagment) StartListening() {
	for i, stream := range em.streams {
		if stream == nil {
			logger.Error().Msgf("Missing stream: %d) ", i)
			continue
		}
		logger.Info().Msgf("Starting stream: %d) %s", i, stream.StreamType())
		go stream.Listen()
	}
}

func (em *StreamManagment) Quit() {
	em.quit <- true
}
