package replicator

import (
	"os"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/estuary"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/cohenjo/replicator/pkg/streams"
	"github.com/cohenjo/replicator/pkg/transform"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
)

type Replicator struct {
	transformer  *transform.TransformationManager
	eventStream  chan *events.RecordEvent
	eventEstuary chan *events.RecordEvent
	quit         chan bool
}

var (
	recordsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "replicator_processed_records_total",
		Help: "The total number of records processed",
	})
	logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
)

/*
Flow is the main replication flow in the system.
It will pull record events from the streams, apply transformations and feed flow to the estuary endpoint manager.
*/
func (replicator *Replicator) Flow() {
	streams.StreamManager.StartListening()
	go estuary.EndpointManager.PublishEvents()

	for {
		select {
		case record := <-replicator.eventStream:
			transformedData := replicator.transformer.Transform(record.Data)
			record.Data = transformedData
			replicator.eventEstuary <- record
			recordsProcessed.Inc()

		case <-replicator.quit:
			return
		}
	}

}

/*
Config is our initialization endpoint.
we setup all system components here.
we will read all configuration files, configure the source and target points, and all transformations required.

*/
func (replicator *Replicator) Config() {

	logger.Info().Msg("starting replicator configuration")
	replicator.quit = make(chan bool)

	// this should be 2 streams with transform in the middle...
	replicator.eventStream = make(chan *events.RecordEvent, config.Global.StreamQueueLength)
	replicator.eventEstuary = make(chan *events.RecordEvent, config.Global.EstuaryQueueLength)

	logger.Info().Msg("Configure streams")
	streams.SetupStreamManager(&replicator.eventStream)
	for _, streamConfig := range config.Global.LegacyStreams {
		streams.StreamManager.NewStream(&streamConfig)
	}

	//set the endpoints
	logger.Info().Msg("Configure endpoints")
	estuary.SetupEndpointManager(&replicator.eventEstuary)
	for _, estuaryConfig := range config.Global.Estuaries {
		estuary.EndpointManager.NewEstuary(&estuaryConfig)

	}

	// define the transformation.
	// note we could easily move this inside the endpoints making the configuration even more crazy
	replicator.transformer = transform.NewTransformer()
	for _, transformConfig := range config.Global.Transforms {
		replicator.transformer.RegisterOperation(transformConfig)
	}

	replicator.transformer.InitializeTransformer()
}

func (re *Replicator) Quit() {
	re.quit <- true
}
