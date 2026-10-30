package main

import (
	"context"
	"example.com/sample/internal/cache"
	"example.com/sample/internal/config"
	"example.com/sample/internal/database"
	"example.com/sample/internal/routes"
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
	dbPing, dbClose, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer dbClose()
	cachePing, cacheClose, err := cache.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer cacheClose()
	app := routes.New(dbPing, cachePing)
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
