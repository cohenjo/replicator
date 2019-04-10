package estuary

import (
	"fmt"
	"os"
	"strings"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/jmoiron/sqlx"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type MySQLEndpoint struct {
	db        string
	tableName string
	// table     *schema.Table
	conn       *sqlx.DB
	insertStmt *sqlx.Stmt
	updateStmt sqlx.Stmt
	deleteStmt sqlx.Stmt
}

// logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

func NewMySQLEndpoint(schema string, collection string) (endpoint MySQLEndpoint) {
	endpoint.db = schema
	endpoint.tableName = collection

	streamUri := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?interpolateParams=true",
		config.Config.MyDBUser,
		config.Config.MyDBPasswd,
		config.Config.MyDBHost, 3306,
		schema,
	)
	conn, err := sqlx.Open("mysql", streamUri)
	if err != nil {
		log.Error().Err(err).Msgf("Error while connecting to source MySQL db")
	}
	endpoint.conn = conn

	// endpoint.insertStmt, _ = conn.Preparex("INSERT INTO " + collection + " VALUES(?)")

	return endpoint
}

func (std MySQLEndpoint) WriteEvent(record *events.RecordEvent) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	row := make(map[string]interface{})
	err := ffjson.Unmarshal(record.Data, &row)
	if err != nil {
		log.Error().Err(err).Msgf("Error while connecting to source MySQL db")
	}

	switch record.Action {
	case "insert":
		// @todo: we can do this on initialization of the endpoint.
		var values strings.Builder
		values.WriteString("insert into ")
		values.WriteString(std.tableName)
		values.WriteString(" values(")
		first := true
		for key, _ := range row {
			// logger.Info().Msgf("Key: %s, Value: %s ", key, value)
			if !first {
				values.WriteString(",")
			}
			values.WriteString(fmt.Sprintf(":%s", key))
			first = false

		}
		values.WriteString(")")
		log.Info().Msgf("Insert stmnt: %s", values.String())
		_, _ = std.conn.NamedExec(values.String(), row)

	case "delete":
	case "update":
		logger.Info().Msgf("update not yes supported , %s", record.Action)

	}

	// logger.Info().Msgf("record: %v", record)
}
