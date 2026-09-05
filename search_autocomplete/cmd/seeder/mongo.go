package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func GetDBConn(uri string) *mongo.Client {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		log.Fatalf("Failed to create MongoDB client: %v", err)
	}

	// 5. Ping the primary deployment to verify the connection is alive
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Fatalf("Could not ping MongoDB: %v", err)
	}

	fmt.Println("Successfully connected and pinged MongoDB!")
	return client
}

func setupMongo(client *mongo.Client, ctx context.Context) *mongo.Collection {
	db := client.Database("search_autocomplete")
	collection := db.Collection("prefix_index")

	_, err := collection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{Key: "prefix", Value: 1},
			},
			Options: options.Index().
				SetUnique(true),
		},
	)

	if err != nil {
		panic(err)
	}

	return collection
}
