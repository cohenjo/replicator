package estuary

import (
	"testing"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
)

func TestIndex(t *testing.T) {

	config.LoadConfiguration()
	// streamer := MySQLStream{}
	ee := NewElasticEndpoint("test", "canal_test")

	record := &events.RecordEvent{
		Action: "insert",
		Data:   []byte(`{"id":6,"output":"hello world"}`),
	}

	ee.WriteEvent(record)

	t.Logf("Finished listenening - look at your terminal ")

}
