package streams

import (
	"testing"

	"github.com/cohenjo/replicator/pkg/config"
)

func TestListen(t *testing.T) {

	config.LoadConfiguration()
	// streamer := MySQLStream{}
	streamer := NewMySQLStream(nil, "test", "canal_test")
	streamer.Listen()

	t.Logf("Finished listenening - look at your terminal ")

}
