package main

import (
	"context"
	"example.com/sample/internal/cache"
	"example.com/sample/internal/config"
	"example.com/sample/internal/database"
	"example.com/sample/internal/routes"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := config.Load()
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get database connection: %w", err)
	}
	defer sqlDB.Close()
	redisClient, err := cache.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer redisClient.Close()
	app := routes.New(db, redisClient)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = app.ShutdownWithContext(shutdownCtx)
	}()
	slog.Info("server started", "port", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}
