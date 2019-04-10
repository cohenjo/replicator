package streams

import (
	"testing"

	"github.com/cohenjo/replicator/pkg/config"
)

func TestListen(t *testing.T) {

	config.LoadConfiguration()
	// streamer := MySQLStream{}
	conf := &config.WaterFlowsConfig{
		Type:       "mysql",
		Host:       "localhost",
		Port:       3306,
		Schema:     "test",
		Collection: "canal_test",
	}
	streamer := NewMySQLStream(nil, conf)
	streamer.Listen()

	t.Logf("Finished listenening - look at your terminal ")

}
