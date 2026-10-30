package routes

import (
	"context"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func check(ctx context.Context, db *gorm.DB, redisClient *redis.Client) error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		if err := sqlDB.PingContext(ctx); err != nil {
			return err
		}
	}
	if redisClient != nil {
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return err
		}
	}
	return nil
}
