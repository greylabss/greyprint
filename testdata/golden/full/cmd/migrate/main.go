package main

import (
	"context"
	"database/sql"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"log/slog"
	_ "modernc.org/sqlite"
	"os"
)

func main() {
	_ = godotenv.Load()
	db, err := sql.Open("sqlite", os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := goose.UpContext(context.Background(), db, "migrations"); err != nil {
		slog.Error("run migrations", "error", err)
		os.Exit(1)
	}
}
