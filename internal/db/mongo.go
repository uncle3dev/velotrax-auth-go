package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/uncle3dev/velotrax-auth-go/internal/model"
)

const DatabaseName = "velotrax"

type DB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func Connect(ctx context.Context, uri string) (*DB, error) {
	opts := options.Client().
		ApplyURI(uri).
		SetServerAPIOptions(options.ServerAPI(options.ServerAPIVersion1)).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(10 * time.Second)

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	return &DB{Client: client, Database: client.Database(DatabaseName)}, nil
}

func (d *DB) Disconnect(ctx context.Context) error {
	return d.Client.Disconnect(ctx)
}

func EnsureIndexes(ctx context.Context, database *mongo.Database) error {
	col := database.Collection(model.CollectionUsers)
	_, err := col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "userName", Value: 1}}, Options: options.Index().SetUnique(true).SetName("idx_users_username_unique")},
		{Keys: bson.D{{Key: "active", Value: 1}}, Options: options.Index().SetName("idx_users_active")},
	})
	if err != nil {
		return fmt.Errorf("users indexes: %w", err)
	}
	return nil
}
