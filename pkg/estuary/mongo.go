package estuary

import (
	"context"
	"fmt"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/pquerna/ffjson/ffjson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoEndpoint struct {
	db             string
	collectionName string
	client         *mongo.Client
	collection     *mongo.Collection
}

func NewMongoEndpoint(streamConfig *config.WaterFlowsConfig) (endpoint MongoEndpoint) {
	// Set client options
	ctx := context.Background()
	uri := fmt.Sprintf("mongodb://%s:%s@%s:27017/admin", config.Config.DBUser, config.Config.DBPasswd, streamConfig.Host)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		logger.Error().Err(err).Msg("connection failure")
	}

	// Check the connection
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		logger.Error().Err(err).Msg("connection ping failure")
	}

	fmt.Println("Connected to MongoDB!")
	collection := client.Database(streamConfig.Schema).Collection(streamConfig.Collection)
	return MongoEndpoint{
		db:             streamConfig.Schema,
		collectionName: streamConfig.Collection,
		client:         client,
		collection:     collection,
	}
}

func (std MongoEndpoint) WriteEvent(record *events.RecordEvent) {

	row := make(map[string]interface{})
	err := ffjson.Unmarshal(record.Data, &row)
	if err != nil {
		logger.Error().Err(err).Msgf("Error while connecting to source MySQL db")
	}

	switch record.Action {
	case "insert":
		insertResult, err := std.collection.InsertOne(context.TODO(), row)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while inserting document")
		}
		fmt.Println("Inserted a single document: ", insertResult.InsertedID)
	case "delete":
	case "update":
		logger.Info().Msgf("update not yes supported , %s", record.Action)

	}

	// logger.Info().Msgf("record: %v", record)
}
