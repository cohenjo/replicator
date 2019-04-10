package streams

import (
	"testing"

	"github.com/cohenjo/replicator/pkg/config"
)

func TestKafkaListen(t *testing.T) {

	config.LoadConfiguration()
	// streamer := MySQLStream{}
	conf := &config.WaterFlowsConfig{
		Type:       "mysql",
		Host:       "localhost",
		Port:       9092,
		Schema:     "db-replicator",
		Collection: "canal_test",
	}
	streamer := NewKafkaStream(nil, conf)
	streamer.Listen()

	t.Logf("Finished listenening - look at your terminal ")

}
