package streams

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pquerna/ffjson/ffjson"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/siddontang/go-mysql/schema"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/juju/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/replication"
)

type MySQLStream struct {
	events    *chan *events.RecordEvent
	db        string
	tableName string
	table     *schema.Table
	syncer    *replication.BinlogSyncer
}

func NewMySQLStream(events *chan *events.RecordEvent, streamConfig *config.WaterFlowsConfig) (stream MySQLStream) {
	stream.events = events
	stream.db = streamConfig.Schema
	stream.tableName = streamConfig.Collection
	return stream
}

func (stream MySQLStream) Configure(events *chan *events.RecordEvent, schema string, collection string) {

	stream.events = events
	stream.db = schema
	stream.tableName = collection
}

func (stream MySQLStream) Listen() {

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	streamUri := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?interpolateParams=true",
		config.Config.MyDBUser,
		config.Config.MyDBPasswd,
		config.Config.MyDBHost, 3306,
		stream.db,
	)
	conn, err := sqlx.Open("mysql", streamUri)
	if err != nil {
		logger.Error().Err(err).Msgf("Error while connecting to source MySQL db")
	}
	gtidSet, err := GetMasterGTIDSet(conn)
	if err != nil {
		logger.Error().Err(err).Msgf("Error while getting GTID set")
	}
	table, err := schema.NewTableFromSqlDB(conn.DB, stream.db, stream.tableName)
	if err != nil {
		logger.Error().Err(err).Msgf("Error while getting table schema")
	}
	stream.table = table

	cfg := replication.BinlogSyncerConfig{
		ServerID: 100,
		Flavor:   "mysql",
		Host:     config.Config.MyDBHost,
		Port:     3306,
		User:     config.Config.MyDBUser,
		Password: config.Config.MyDBPasswd,
	}
	stream.syncer = replication.NewBinlogSyncer(cfg)

	// or you can start a gtid replication like
	if stream.syncer == nil {
		logger.Error().Msgf("This is nil????")
	}
	streamer, err := stream.syncer.StartSyncGTID(gtidSet)
	if err != nil {
		logger.Error().Err(err).Msgf("Error while starting sync")
	}
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

		switch ev.Event.(type) {
		case *replication.RowsEvent:
			// we only focus row based event
			err = stream.handleRowsEvent(ev)
			if err != nil {
				logger.Error().Err(err).Msgf("Failed to send event")
			}
			continue
		default:
			continue
		}

	}

}

func GetMasterGTIDSet(conn *sqlx.DB) (mysql.GTIDSet, error) {

	query := "SELECT @@GLOBAL.GTID_EXECUTED"

	var gx string
	err := conn.Get(&gx, query)

	if err != nil {
		return nil, errors.Trace(err)
	}
	gset, err := mysql.ParseGTIDSet(mysql.MySQLFlavor, gx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return gset, nil
}

func (stream MySQLStream) StreamType() string {
	return "MySQL"
}

func (stream MySQLStream) handleRowsEvent(e *replication.BinlogEvent) error {
	ev := e.Event.(*replication.RowsEvent)

	// Caveat: table may be altered at runtime.
	schema := string(ev.Table.Schema)
	table := string(ev.Table.Table)

	// only handle events on our table
	if stream.tableName != table || stream.db != schema {
		return nil
	}

	var action string
	switch e.Header.EventType {
	case replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:
		action = events.InsertAction
	case replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
		action = events.DeleteAction
	case replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2:
		action = events.UpdateAction
	default:
		return errors.Errorf("%s not supported now", e.Header.EventType)
	}

	switch action {
	case events.UpdateAction:
		log.Error().Msg("don't support update yet")
	case events.InsertAction, events.DeleteAction:
		for _, row := range ev.Rows {

			toJson := make(map[string]interface{})
			for i, val := range row {
				toJson[stream.table.Columns[i].Name] = val
			}
			data, err := ffjson.Marshal(toJson)
			if err != nil {
				log.Error().Err(err).Msg("could not marshel the row")
				continue
			}
			record := &events.RecordEvent{
				Action:     action,
				Schema:     schema,
				Collection: table,
				Data:       data,
			}

			if stream.events != nil {
				log.Debug().Str("action", action).Msgf("row: %s", string(data))
				*stream.events <- record
			}

		}
	}

	return nil
}
