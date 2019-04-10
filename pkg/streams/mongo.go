package streams

import (
	"context"
	"fmt"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/rs/zerolog/log"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoStream struct {
	events     *chan *events.RecordEvent
	db         string
	collection string
}

func NewMongoStream(events *chan *events.RecordEvent, streamConfig *config.WaterFlowsConfig) (stream MongoStream) {
	stream.events = events
	stream.db = streamConfig.Schema
	stream.collection = streamConfig.Collection
	return stream
}

func (stream MongoStream) StreamType() string {
	return "Mongo"
}

func (stream MongoStream) Listen() {

	// ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	ctx := context.Background()
	uri := fmt.Sprintf("mongodb://%s:%s@%s:27017/admin", config.Config.DBUser, config.Config.DBPasswd, config.Config.DBHost)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Error().Err(err).Msgf("error failed to connect: %v", err)
	}
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Error().Err(err).Msgf("error failed to ping: %v", err)
	}
	collection := client.Database(stream.db).Collection(stream.collection)

	cs, err := collection.Watch(ctx, mongo.Pipeline{})
	if err != nil {
		log.Error().Err(err).Msgf("error failed to watch: %v \n", err)
	}

	// defer cs.Close(ctx)

	for {

		ok := cs.Next(ctx)
		if ok {
			next := cs.Current
			action := next.Lookup("operationType").String()
			data := []byte(next.Lookup("fullDocument").String())

			record := &events.RecordEvent{
				Action:     action,
				Schema:     stream.db,
				Collection: stream.collection,
				Data:       data,
			}

			if stream.events != nil {
				log.Debug().Str("action", action).Msgf("row: %s", string(data))
				*stream.events <- record
			}
		} else {
			time.Sleep(100 * time.Millisecond)
			log.Debug().Str("action", "sleep").Msgf("got nothing: %s \n", cs.Err())
		}
	}

}
