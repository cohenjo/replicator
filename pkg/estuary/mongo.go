package estuary

import (
	"context"
	"fmt"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/pquerna/ffjson/ffjson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/mgo.v2/bson"
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
		logger.Error().Err(err).Msgf("Error while unmarshal record")
		return
	}
	logger.Info().Str("action", record.Action).Str("name", std.collection.Name()).Msgf("write event: %+v", row)
	var key events.RecordKey
	err = ffjson.Unmarshal(record.OldData, &key)
	if err != nil {
		logger.Error().Err(err).Msgf("Error while Unmarshal key")
		return
	}

	row["id"] = key.ID

	switch record.Action {
	case "insert":
		insertResult, err := std.collection.InsertOne(context.TODO(), row)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while inserting document")
			return
		}
		logger.Info().Msgf("Inserted a single document: %s", insertResult.InsertedID)
	case "delete":

		filter, err := bson.Marshal(key)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while Marshal filter")
			return
		}

		deleteResult, err := std.collection.DeleteMany(context.TODO(), filter)
		if err != nil {
			logger.Error().Err(err).Msgf("Failed to delete record")
			return
		}
		logger.Info().Int("DeletedCount", int(deleteResult.DeletedCount)).Msg("record deleted properly")
	case "update":
		filter, err := bson.Marshal(key)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while Marshal filter")
			return
		}

		logger.Info().Msgf("update not yes supported , %s", record.Action)
		updateResult, err := std.collection.UpdateOne(context.TODO(), filter, row)
		if err != nil {
			logger.Error().Err(err).Msgf("Failed to update record")
			return
		}
		logger.Info().Int("MatchedCount", int(updateResult.MatchedCount)).Int("ModifiedCount", int(updateResult.ModifiedCount)).Msg("record Updated properly")
	}

	// logger.Info().Msgf("record: %v", record)
}
