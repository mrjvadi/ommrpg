// Package metrics counts events in-process and flushes them once per second
// into Dragonfly sliding-window counters (pkg/hot counter.lua), so the admin
// panel can show live rates across all replicas at negligible cost.
package metrics

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mrjvadi/ommrpg/backend/pkg/hot"
)

const Window = 300 // seconds kept in each counter

type Counters struct {
	mu  sync.Mutex
	n   map[string]int64
	rdb *redis.Client
}

func New(ctx context.Context, rdb *redis.Client) *Counters {
	c := &Counters{n: map[string]int64{}, rdb: rdb}
	go c.loop(ctx)
	return c
}

// Inc adds to a named counter (e.g. "kills", "moves", "rpc").
func (c *Counters) Inc(name string, v int64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.n[name] += v
	c.mu.Unlock()
}

func (c *Counters) loop(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.mu.Lock()
			batch := c.n
			c.n = map[string]int64{}
			c.mu.Unlock()
			for k, v := range batch {
				_, _ = hot.Count(ctx, c.rdb, Key(k), v, Window)
			}
		}
	}
}

func Key(name string) string { return "metric:" + name }

// Rate returns the sum of a counter over the last `window` seconds.
func Rate(ctx context.Context, rdb *redis.Client, name string, window int) int64 {
	m, err := rdb.HGetAll(ctx, Key(name)).Result()
	if err != nil {
		return 0
	}
	now := time.Now().Unix()
	var sum int64
	for sec, v := range m {
		s, err1 := strconv.ParseInt(sec, 10, 64)
		n, err2 := strconv.ParseInt(v, 10, 64)
		if err1 == nil && err2 == nil && s > now-int64(window) {
			sum += n
		}
	}
	return sum
}
