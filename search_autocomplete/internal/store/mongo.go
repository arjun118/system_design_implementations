package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/arjun118/autocomplete/internal/suggest"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoStore struct {
	collection *mongo.Collection
}

func NewMongoStore(uri string) *MongoStore {
	client := GetDBConn(uri)
	db := client.Database("search_autocomplete")
	collection := db.Collection("prefix_index")
	return &MongoStore{
		collection: collection,
	}
}

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

func (m *MongoStore) Get(prefix string) ([]suggest.Reco, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	res := m.collection.FindOne(ctx, bson.M{
		"prefix": prefix,
	})
	var doc suggest.Recos
	err := res.Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return doc.Top, nil
}

func (m *MongoStore) GetTopPrefixes(n int) ([]suggest.Recos, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	log.Printf("[SEED FETCH] fetching docs from mongodb\n")
	findOptions := options.Find().
		SetSort(bson.D{{Key: "top.0.freq", Value: -1}}).
		SetLimit(int64(n))
	cursor, err := m.collection.Find(ctx, bson.D{}, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []suggest.Recos
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}
