package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Dragonfly connects to Dragonfly (Redis protocol) used for hot state:
// positions, hp, cooldowns, monster state, rate limits, dungeon instances.
func Dragonfly(ctx context.Context, url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	c := redis.NewClient(opt)
	var lastErr error
	for i := 0; i < 30; i++ {
		if lastErr = c.Ping(ctx).Err(); lastErr == nil {
			return c, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("dragonfly: %w", lastErr)
}
