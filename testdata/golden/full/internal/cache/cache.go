package cache

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, address string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{Addr: address})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return client, nil
}
