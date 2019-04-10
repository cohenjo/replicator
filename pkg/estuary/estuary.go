package estuary

// estuary means "mouth of river"
// Noun, the tidal mouth of a large river, where the tide meets the stream

// we will have here implementations to write an event to an output server as defined in the configuration.

import (
	"fmt"
	"os"

	"github.com/cohenjo/replicator/pkg/events"
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

var EndpointManager *EndpointManagment

func SetupEndpointManager(events *chan *events.RecordEvent) {
	EndpointManager = &EndpointManagment{
		recordEvents: events,
		endpoints:    make([]Endpoint, 0),
		quit:         make(chan bool),
	}
}

func (em *EndpointManagment) NewEstuary(streamType, schema, collection string) {
	var endpoint Endpoint
	switch streamType {
	case "MYSQL":
		endpoint = NewMySQLEndpoint(schema, collection)
	case "MONGO":
		endpoint = NewMongoEndpoint(schema, collection)
	case "KAFKA":
		endpoint = NewKafkaEndpoint(schema, collection)
	case "ELASTIC":
		endpoint = NewElasticEndpoint(schema, collection)
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
