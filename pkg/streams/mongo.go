package streams

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cohenjo/replicator/pkg/config"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func MongoStream() {
	breakSig := make(chan os.Signal, 1)
	signal.Notify(breakSig, syscall.SIGINT, syscall.SIGTERM)

	stopSignal := false

	config.LoadConfiguration()

	go func() {
		<-breakSig
		stopSignal = true
	}()

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	uri := fmt.Sprintf("mongodb://%s:%s@%s:27017/admin", config.Config.DBUser, config.Config.DBPasswd, config.Config.DBHost)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		fmt.Printf("error failed to connect: %v", err)
	}
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		fmt.Printf("error failed to ping: %v", err)
	}
	collection := client.Database("testings").Collection("numbers")

	cs, err := collection.Watch(ctx, mongo.Pipeline{})
	if err != nil {
		fmt.Printf("error failed to watch: %v \n", err)
	}

	defer cs.Close(ctx)

	for !stopSignal {

		ok := cs.Next(ctx)
		if ok {
			next := cs.Current

			fmt.Printf("got change: %v \n", next)
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}

}
