package main

import (
	"context"
	"fmt"
	"time"

	"github.com/mongodb/mongo-go-driver/mongo"
	"github.com/mongodb/mongo-go-driver/mongo/options"
	"github.com/mongodb/mongo-go-driver/mongo/readpref"
)

func main() {

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	err = client.Ping(ctx, readpref.Primary())
	collection := client.Database("testing").Collection("numbers")

	cs, err := collection.Watch(ctx, mongo.Pipeline{})

	defer cs.Close(ctx)

	ok := cs.Next(ctx)
	next := cs.Current

	fmt.Printf("got change: %+v", next)

}
