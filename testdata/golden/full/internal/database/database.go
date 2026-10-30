package database

import (
	"context"
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func Connect(ctx context.Context, url string) (func(context.Context) error, func(), error) {
	db, err := gorm.Open(sqlite.Open(url), &gorm.Config{})
	if err != nil {
		return nil, nil, fmt.Errorf("connect database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("get database connection: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("ping database: %w", err)
	}
	return func(ctx context.Context) error { return sqlDB.PingContext(ctx) }, func() { _ = sqlDB.Close() }, nil
}
