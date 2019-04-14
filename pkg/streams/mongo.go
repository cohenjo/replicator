package streams

import (
	"context"
	"fmt"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/pquerna/ffjson/ffjson"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoStream struct {
	events     *chan *events.RecordEvent
	config     *config.WaterFlowsConfig
	db         string
	collection string
}

func NewMongoStream(events *chan *events.RecordEvent, streamConfig *config.WaterFlowsConfig) (stream MongoStream) {
	stream.events = events
	stream.db = streamConfig.Schema
	stream.collection = streamConfig.Collection
	stream.config = streamConfig
	return stream
}

func (stream MongoStream) StreamType() string {
	return "Mongo"
}

func (stream MongoStream) Listen() {

	// ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	ctx := context.Background()
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%d/admin", config.Config.DBUser, config.Config.DBPasswd, stream.config.Host, stream.config.Port)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		logger.Error().Err(err).Msgf("error failed to connect: %v", err)
	}
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		logger.Error().Err(err).Msgf("error failed to ping: %v", err)
	}
	collection := client.Database(stream.db).Collection(stream.collection)

	cs, err := collection.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	if err != nil {
		logger.Error().Err(err).Msgf("error failed to watch: \n")
	}

	// defer cs.Close(ctx)
	// resumeToken := next.Lookup("_id").Document()
	// cs, err := coll.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetResumeAfter(resumeToken))

	for {

		ok := cs.Next(ctx)
		if ok {
			next := cs.Current
			action := next.Lookup("operationType").String()

			toJsonKey := make(map[string]interface{})
			next.Lookup("documentKey").Unmarshal(&toJsonKey)
			documentKey, err := ffjson.Marshal(toJsonKey)
			if err != nil {
				logger.Error().Err(err).Msgf("error marshel doc key:  ")
			}

			toJson := make(map[string]interface{})
			next.Lookup("fullDocument").Unmarshal(&toJson)
			data, err := ffjson.Marshal(toJson)
			if err != nil {
				logger.Error().Err(err).Msgf("error enmarshal:  ")
			}

			record := &events.RecordEvent{
				Action:     action,
				Schema:     stream.db,
				Collection: stream.collection,
				OldData:    documentKey,
				Data:       data,
			}

			if stream.events != nil {
				logger.Debug().Str("action", action).Msgf("row: %s key: %s", string(data), string(documentKey))
				*stream.events <- record
				recordsRecieved.Inc()
			}
		} else {
			time.Sleep(100 * time.Millisecond)
			logger.Debug().Str("action", "sleep").Msgf("got nothing: %s \n", cs.Err())
		}
	}

}
