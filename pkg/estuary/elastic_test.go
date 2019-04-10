package estuary

import (
	"testing"

	"github.com/cohenjo/replicator/pkg/config"
)

func TestIndex(t *testing.T) {

	config.LoadConfiguration()
	// streamer := MySQLStream{}
	ee := NewElasticEndpoint("test", "canal_test")
	ee.WriteEvent(nil)

	t.Logf("Finished listenening - look at your terminal ")

}
