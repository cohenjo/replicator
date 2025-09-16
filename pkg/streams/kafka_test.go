package streams

import (
	"testing"

	"github.com/cohenjo/replicator/pkg/config"
)

func TestKafkaListen(t *testing.T) {

	// Don't call LoadConfiguration() in tests - it hangs waiting for config file
	// config.LoadConfiguration()
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
