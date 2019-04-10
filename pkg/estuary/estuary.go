package estuary

// estuary means "mouth of river"
// Noun, the tidal mouth of a large river, where the tide meets the stream

// we will have here implementations to write an event to an output server as defined in the configuration.

import (
	"fmt"
	"os"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
)

type Endpoint interface {
	WriteEvent(record *events.RecordEvent)
}

func WriteEndpoints(record events.RecordEvent) {
	fmt.Printf("write to all registered endpoints: %v \n", record)
}

type EndpointManagment struct {
	recordEvents *chan *events.RecordEvent
	endpoints    []Endpoint
	quit         chan bool
}

var (
	EndpointManager *EndpointManagment
	logger          = zerolog.New(os.Stderr).With().Timestamp().Logger()
	recordsSent     = promauto.NewCounter(prometheus.CounterOpts{
		Name: "replicator_sent_records_total",
		Help: "The total number of records sent",
	})
)

func SetupEndpointManager(events *chan *events.RecordEvent) {
	EndpointManager = &EndpointManagment{
		recordEvents: events,
		endpoints:    make([]Endpoint, 0),
		quit:         make(chan bool),
	}
}

func (em *EndpointManagment) NewEstuary(streamConfig *config.WaterFlowsConfig) {
	var endpoint Endpoint
	switch streamConfig.Type {
	case "MYSQL":
		endpoint = NewMySQLEndpoint(streamConfig)
	case "MONGO":
		endpoint = NewMongoEndpoint(streamConfig)
	case "KAFKA":
		endpoint = NewKafkaEndpoint(streamConfig)
	case "ELASTIC":
		endpoint = NewElasticEndpoint(streamConfig)
	case "STDOUT":
		endpoint = StdoutEndpoint{}

	}
	em.endpoints = append(em.endpoints, endpoint)
}

func (em *EndpointManagment) RegisterEndpoint(endpoint Endpoint) {
	em.endpoints = append(em.endpoints, endpoint)
}

func (em *EndpointManagment) PublishEvents() {
	for {
		select {
		case record := <-*em.recordEvents:
			for _, endpoint := range em.endpoints {
				endpoint.WriteEvent(record)
			}
			recordsSent.Inc()
		case <-em.quit:
			return
		}
	}

}

func (em *EndpointManagment) PublishEvent(record *events.RecordEvent) {
	*em.recordEvents <- record
}

func (em *EndpointManagment) Quit() {
	em.quit <- true
}

type StdoutEndpoint struct {
}

func (std StdoutEndpoint) WriteEvent(record *events.RecordEvent) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	logger.Info().Msgf("record: %s", string(record.Data))
}
