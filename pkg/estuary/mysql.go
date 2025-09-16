package estuary

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/jmoiron/sqlx"
	"github.com/pquerna/ffjson/ffjson"
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

func NewMySQLEndpoint(streamConfig *config.WaterFlowsConfig) (endpoint MySQLEndpoint) {
	endpoint.db = streamConfig.Schema
	endpoint.tableName = streamConfig.Collection

	streamUri := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?interpolateParams=true",
		config.Global.MyDBUser,
		config.Global.MyDBPasswd,
		streamConfig.Host, streamConfig.Port,
		streamConfig.Schema,
	)
	conn, err := sqlx.Open("mysql", streamUri)
	if err != nil {
		logger.Error().Err(err).Msgf("Error while connecting to source MySQL db")
	}
	endpoint.conn = conn

	// endpoint.insertStmt, _ = conn.Preparex("INSERT INTO " + collection + " VALUES(?)")

	return endpoint
}

func (std MySQLEndpoint) WriteEvent(record *events.RecordEvent) {

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
		for key, value := range row {
			logger.Info().Msgf("Key: %s, type ", key)
			switch v := value.(type) {
			case string:
				logger.Info().Msgf("Key: %s, value %s - string ", key, v)
				row[key] = v
			default:
				logger.Info().Msgf("Key: %s, is not string", key)
			}
			if !first {
				values.WriteString(",")
			}
			values.WriteString(fmt.Sprintf(":%s", key))
			// values.WriteString("?")
			first = false

		}

		values.WriteString(")")
		row["id"], _ = hex.DecodeString(row["id"].(string))
		logger.Info().Msgf("Insert stmnt: %s, record: %v", values.String(), row)
		tx := std.conn.MustBegin()
		res, err := tx.NamedExec(values.String(), row)
		if err != nil {
			log.Error().Err(err).Msgf("Error inserting to MySQL. res: %v", res)
		}
		err = tx.Commit()
		if err != nil {
			log.Error().Err(err).Msgf("Commit error")
		}

	case "delete":
	case "update":
		logger.Info().Msgf("update not yes supported , %s", record.Action)

	}

	// logger.Info().Msgf("record: %v", record)
}
