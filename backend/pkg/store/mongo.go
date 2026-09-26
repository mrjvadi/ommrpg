package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Mongo connects to MongoDB (documents: history, world-firsts, lore).
func Mongo(ctx context.Context, uri, db string) (*mongo.Database, error) {
	c, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		return nil, err
	}
	var lastErr error
	for i := 0; i < 20; i++ {
		if lastErr = c.Ping(ctx, nil); lastErr == nil {
			return c.Database(db), nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("mongo: %w", lastErr)
}
