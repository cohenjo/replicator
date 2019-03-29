package streams

import (
	"context"
	"time"

	"github.com/siddontang/go-mysql/client"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/juju/errors"
	"github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/replication"

	"os"
)

func MySQLStream() {

	cfg := replication.BinlogSyncerConfig{
		ServerID: 100,
		Flavor:   "mysql",
		Host:     config.Config.MyDBHost,
		Port:     3306,
		User:     config.Config.MyDBUser,
		Password: config.Config.MyDBPasswd,
	}
	syncer := replication.NewBinlogSyncer(cfg)

	conn, _ := client.Connect(config.Config.MyDBHost+":3306", config.Config.MyDBUser, config.Config.MyDBPasswd, "test")

	gtidSet, _ := GetMasterGTIDSet(conn)
	// or you can start a gtid replication like
	streamer, _ := syncer.StartSyncGTID(gtidSet)
	// the mysql GTID set likes this "de278ad0-2106-11e4-9f8e-6edd0ca20947:1-2"
	// the mariadb GTID set likes this "0-1-100"

	// or we can use a timeout context
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		ev, err := streamer.GetEvent(ctx)
		cancel()

		if err == context.DeadlineExceeded {
			// meet timeout
			continue
		}

		ev.Dump(os.Stdout)
	}

}

func GetMasterGTIDSet(conn *client.Conn) (mysql.GTIDSet, error) {

	query := "SELECT @@GLOBAL.GTID_EXECUTED"

	rr, err := conn.Execute(query)
	if err != nil {
		return nil, errors.Trace(err)
	}
	gx, err := rr.GetString(0, 0)
	if err != nil {
		return nil, errors.Trace(err)
	}
	gset, err := mysql.ParseGTIDSet(mysql.MySQLFlavor, gx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return gset, nil
}
