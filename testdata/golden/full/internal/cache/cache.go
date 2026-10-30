package cache

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, address string) (func(context.Context) error, func(), error) {
	client := redis.NewClient(&redis.Options{Addr: address})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("connect redis: %w", err)
	}
	return func(ctx context.Context) error { return client.Ping(ctx).Err() }, func() { _ = client.Close() }, nil
}
