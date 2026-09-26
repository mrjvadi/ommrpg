package main

import (
	"context"

	"github.com/mrjvadi/ommrpg/backend/pkg/hot"
	"github.com/mrjvadi/ommrpg/backend/pkg/metrics"
)

func hotCount(ctx context.Context, a *app, name string, n int64) (int64, error) {
	return hot.Count(ctx, a.rdb, metrics.Key(name), n, metrics.Window)
}
