package streams

import (
	"testing"

	"github.com/cohenjo/replicator/pkg/config"
)

func TestKafkaListen(t *testing.T) {

	config.LoadConfiguration()
	// streamer := MySQLStream{}
	streamer := NewKafkaStream(nil, "db-replicator", "canal_test")
	streamer.Listen()

	t.Logf("Finished listenening - look at your terminal ")

}
