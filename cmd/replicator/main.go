package main

import (
	"context"
	"fmt"
	"time"

	"github/cohenjo/replicator/pkg/config"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func main() {

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	uri := fmt.Sprintf("mongodb://%s:%s@%s:27017/admin", config.Config.DBUser, config.Config.DBPasswd, config.Config.DBHost)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	collection := client.Database("testings").Collection("numbers")

	cs, err := collection.Watch(ctx, mongo.Pipeline{})
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	defer cs.Close(ctx)
	fmt.Printf("error: %v", cs)
	ok := cs.Next(ctx)
	if !ok {
		fmt.Printf("error: %v", ok)
	}
	next := cs.Current

	fmt.Printf("got change: %+v", next)

}
