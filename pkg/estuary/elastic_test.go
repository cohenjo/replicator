package estuary

import (
	"testing"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
)

func TestIndex(t *testing.T) {

	// Don't call LoadConfiguration() in tests - just use test config directly
	// config.LoadConfiguration()
	// streamer := MySQLStream{}
	
	// Create a test config for the endpoint
	testConfig := &config.WaterFlowsConfig{
		Host: "localhost",
		Port: 9200,
	}
	ee := NewElasticEndpoint(testConfig)

	record := &events.RecordEvent{
		Action: "insert",
		Data:   []byte(`{"id":6,"output":"hello world"}`),
	}

	ee.WriteEvent(record)

	t.Logf("Finished listenening - look at your terminal ")

}
