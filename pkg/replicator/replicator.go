package replicator

import (
	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/estuary"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/cohenjo/replicator/pkg/streams"
	"github.com/cohenjo/replicator/pkg/transform"
)

type Replicator struct {
	transformer  *transform.TransformationManager
	eventStream  chan *events.RecordEvent
	eventEstuary chan *events.RecordEvent
	quit         chan bool
}

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

	replicator.quit = make(chan bool)

	// this should be 2 streams with transform in the middle...
	replicator.eventStream = make(chan *events.RecordEvent, 1000)
	replicator.eventEstuary = make(chan *events.RecordEvent, 1000)

	streams.SetupStreamManager(&replicator.eventStream)
	for _, streamConfig := range config.Config.Streams {
		streams.StreamManager.NewStream(streamConfig.DBType, streamConfig.Schema, streamConfig.Collection)

	}

	//set the endpoints
	estuary.SetupEndpointManager(&replicator.eventEstuary)
	for _, estuaryConfig := range config.Config.Estuaries {
		estuary.EndpointManager.NewEstuary(estuaryConfig.DBType, estuaryConfig.Schema, estuaryConfig.Collection)

	}

	// define the transformation.
	// note we could easily move this inside the endpoints making the configuration even more crazy
	replicator.transformer = transform.NewTransformer()
	replicator.transformer.RegisterOperation(transform.Operation{
		Operation: "shift",
		Spec:      map[string]interface{}{"output": "t", "id": "id"},
	})
	replicator.transformer.InitializeTransformer()
}

func (re *Replicator) Quit() {
	re.quit <- true
}
