package streams

import (
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/rs/zerolog/log"
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

var StreamManager *StreamManagment

func SetupStreamManager(events *chan *events.RecordEvent) {
	StreamManager = &StreamManagment{
		events:  events,
		streams: make([]Stream, 0),
		quit:    make(chan bool),
	}
}

func (sm *StreamManagment) NewStream(streamType, schema, collection string) {
	var stream Stream
	switch streamType {
	case "MYSQL":
		stream = NewMySQLStream(sm.events, schema, collection)
	case "MONGO":
		stream = NewMongoStream(sm.events, schema, collection)
	case "KAFKA":
		stream = NewKafkaStream(sm.events, schema, collection)
	}
	sm.streams = append(sm.streams, stream)
}

func (sm *StreamManagment) registerStreams(stream Stream) {
	sm.streams = append(sm.streams, stream)
}

func (em *StreamManagment) StartListening() {
	for i, stream := range em.streams {
		log.Info().Msgf("Starting stream: %d) %s", i, stream.StreamType())
		go stream.Listen()
	}
}

func (em *StreamManagment) Quit() {
	em.quit <- true
}
