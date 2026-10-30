# Greyprint — Spec

> GreyLabs · CLI that scaffolds minimal, standard Go backends.
> Status: v0.1 plan · Last updated: 2026-10-30

## 1. Goal

Run one command, answer a few questions, get a Go backend that builds and runs immediately, with the layout an experienced Go developer would set up by hand and nothing more.

**Principles**

1. **Bare minimum.** Only what the user chose. A hello-world API with the chosen pieces wired together, not a framework.
2. **Standard layout.** Follows the Go team's guide "Organizing a Go module" (go.dev/doc/modules/layout). Not `golang-standards/project-layout`.
3. **Readable for years.** No comments in generated code. Names and structure explain it.
4. **Works first time.** Every combination of options builds, vets and runs. Tested in CI.
5. **Agent-friendly.** Every prompt has a flag, so humans and AI agents can both use it.

**Non-goals:** auth, users, CRUD generators, frontends, Docker files, Kubernetes, opinionated architecture (no "clean architecture" layers, no repositories/services/usecases split).

## 2. Commands

```
greyprint new [name]        interactive scaffold (prompts for anything not given as a flag)
greyprint version
```

### 2.1 Flags for `new`

| Flag | Values | Default |
| --- | --- | --- |
| `--module` | Go module path | `github.com/<git user.name or "me">/<name>` |
| `--router` | `stdlib`, `chi`, `gin`, `echo`, `fiber` | `stdlib` |
| `--db` | `none`, `postgres`, `mysql`, `sqlite` | `none` |
| `--access` | `driver`, `sqlc`, `gorm` | `driver` (ignored when `--db none`) |
| `--redis` | `true` / `false` | `false` |
| `--greyship` | `true` / `false` | `true` |
| `--dir` | output directory | `./<name>` |
| `--yes`, `-y` | accept defaults for anything not given, no prompts | off |
| `--no-install` | skip `go mod tidy` / `go get` | off |
| `--no-git` | skip `git init` | off |
| `--dry-run` | print the file tree, write nothing | off |
| `--json` | machine-readable output (implies no prompts; missing values use defaults) | off |

If `name` is missing and prompts are off, fail with a clear error.

### 2.2 Interactive flow

Prompts appear only for values not given as flags, in this order:

1. Project name
2. Module path (pre-filled with the default)
3. Router: Standard library (net/http) · chi · Gin · Echo · Fiber
4. Database: None · PostgreSQL · MySQL · SQLite
5. Database access (only if a database was picked): Driver only · sqlc · GORM
6. Redis? (y/N)
7. Greyship-ready? (Y/n)
8. Summary of choices → confirm

### 2.3 Validation

- Name: lowercase letters, digits, `-`, `_`; starts with a letter; max 64 chars.
- Output dir must not exist, or must be empty.
- `--access` other than `driver` with `--db none` → error naming both flags.
- Unknown flag values → error listing the valid values.

### 2.4 After writing files

Unless disabled:

1. `go.mod` is rendered directly (module path, Go version, `require` lines at pinned versions), so there is no `go mod init` step.
2. `go mod tidy` downloads modules and writes `go.sum`. With `--no-install` this is skipped and the next steps tell the user to run it.
3. `git init` (no commit). `.gitignore` is already rendered.
4. Print next steps:

```
✓ Created myapp

  cd myapp
  cp .env.example .env
  make run

  Deploy:  grey deploy      (only if Greyship-ready)
```

If `go` isn't installed or a step fails, files stay written, the error says which step failed and how to rerun it by hand.

`--json` output: `{"name", "dir", "module", "options": {...}, "files": [...], "steps": [{"name", "ok", "error"}]}`.

## 3. Generated project

### 3.1 Layout

```
myapp/
  cmd/
    api/main.go
    migrate/main.go          only with a database and --greyship
  internal/
    config/config.go
    routes/routes.go
    routes/health.go
    database/database.go     only with a database
    cache/cache.go           only with --redis
  migrations/
    00001_init.sql           only with a database
  sqlc.yaml                  only with --access sqlc
  internal/database/queries/ only with --access sqlc
  .env.example
  .gitignore
  Makefile
  README.md
  go.mod
```

### 3.2 Packages

- **`config`**: loads `.env` with godotenv (missing file is fine), returns a `Config` struct. Adding a variable = one field + one line.
- **`routes`**: builds the router, registers routes, holds handlers. Ships with `GET /health` returning `{"status":"ok"}` (and pings the DB / Redis when present).
- **`database`**: opens and pings the connection, returns it. Nothing else.
- **`cache`**: opens and pings the Redis client, returns it.
- **`cmd/api`**: loads config, connects, builds routes, runs the server, shuts down gracefully on SIGINT/SIGTERM (10s timeout).
- **`cmd/migrate`**: runs goose migrations from `migrations/` and exits. Greyship runs `cmd/migrate` as the release step automatically.

### 3.3 Example: `cmd/api/main.go` (stdlib + postgres)

This is the quality bar for every template.

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/myapp/internal/config"
	"example.com/myapp/internal/database"
	"example.com/myapp/internal/routes"
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
	defer db.Close()

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           routes.New(db),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	slog.Info("server started", "port", cfg.Port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
```

### 3.4 Example: `internal/config/config.go`

```go
package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

Fields appear only for what was chosen (`DatabaseURL` with a database, `RedisURL` with Redis).

### 3.5 Files

- **`.env.example`**: `PORT=8080` plus `DATABASE_URL` / `REDIS_URL` placeholders for what was chosen. Local defaults that work with a standard local install (e.g. `postgres://app:app@localhost:5432/myapp?sslmode=disable`, `file:myapp.db` for SQLite).
- **`.gitignore`**: `.env`, `.env.local`, `.env.prod`, `/bin`, `*.db` (SQLite).
- **`Makefile`**: `run`, `build`, `test`, `tidy`; plus `migrate` with a database; plus `sqlc` with sqlc.
- **`README.md`**: one paragraph, the commands to run it, the layout tree. Short.
- **`migrations/00001_init.sql`**: goose format, creates nothing heavy — an `items` table with `id` and `created_at` is enough to prove it works. sqlc queries (if chosen) match it.

### 3.6 Greyship-ready (`--greyship`, default on)

- Layout already matches what Greyship detects: `cmd/api` → web service, `cmd/migrate` → release step. No `greyship.toml` is generated.
- `.gitignore` covers `.env.prod`.
- Server reads `PORT` (Greyship injects it).
- Everything builds with `CGO_ENABLED=0` (hence the pure-Go SQLite driver).
- README's last section: `grey deploy`.

When off: no `cmd/migrate` (the Makefile `migrate` target calls goose directly), no deploy section.

## 4. Option → dependency map

| Choice | Packages added |
| --- | --- |
| always | `github.com/joho/godotenv` |
| router `chi` | `github.com/go-chi/chi/v5` |
| router `gin` | `github.com/gin-gonic/gin` |
| router `echo` | `github.com/labstack/echo/v4` |
| router `fiber` | `github.com/gofiber/fiber/v2` |
| postgres + driver/sqlc | `github.com/jackc/pgx/v5` (pgxpool) |
| mysql + driver/sqlc | `github.com/go-sql-driver/mysql` |
| sqlite + driver/sqlc | `modernc.org/sqlite` (pure Go, CGO-free) |
| gorm + postgres | `gorm.io/gorm`, `gorm.io/driver/postgres` |
| gorm + mysql | `gorm.io/gorm`, `gorm.io/driver/mysql` |
| gorm + sqlite | `gorm.io/gorm`, `github.com/glebarez/sqlite` (pure Go) |
| any database | `github.com/pressly/goose/v3` |
| redis | `github.com/redis/go-redis/v9` |

`sqlc` is a code generator, not a module: generated projects include `sqlc.yaml`, queries and the **already generated** Go code so the project builds without sqlc installed. `make sqlc` regenerates.

Fiber runs on fasthttp, not `net/http`: its `main.go` uses `app.Listen` / `app.ShutdownWithContext` instead of `http.Server`. Same structure, same behaviour.

All versions are pinned in `internal/versions/versions.go`.

## 5. Greyprint's own architecture

```
flags + prompts ─► plan.Options ─► plan.Validate ─► render.Tree ─► write files ─► install steps
                     (pure)          (pure)           (pure)        (cli)          (install)
```

- `plan.Options`: one struct holding every choice.
- `render.Tree(opts) (map[string][]byte, error)`: path → contents. Templates are selected by option (e.g. `routes/routes.go.chi.tmpl`), rendered with `text/template`, then run through `go/format` for `.go` files.
- Writing and installing happen only after rendering fully succeeds, so a template error never leaves a half-written project.

## 6. Distribution

- `go install <module>/cmd/greyprint@latest`
- GoReleaser binaries for Linux, macOS and Windows (amd64 + arm64) on GitHub Releases.
- Module path: `github.com/greylabss/greyprint`.
- Install scripts in the repository fetch the latest platform archive from GitHub Releases and verify its SHA-256 checksum before installation.
- A tag-triggered GitHub Actions workflow publishes Linux, macOS, and Windows binaries for amd64 and arm64 with GoReleaser.

## 7. Milestones

| # | Scope | Done when |
| --- | --- | --- |
| M1 | `plan` + `render` + `stdlib` router, no database, `--yes` / flags only | Golden test passes; generated project runs `make run` and `/health` answers |
| M2 | All five routers | Matrix test green for routers × no DB |
| M3 | Databases (postgres, mysql, sqlite) with driver access + goose migrations + `cmd/migrate` | Matrix green; postgres project migrates against a real local DB |
| M4 | sqlc and GORM access | Matrix green for every DB × access combination |
| M5 | Redis, Greyship-ready toggle | Matrix green for the full option set |
| M6 | Interactive prompts, install steps, next-steps output, `--json`, `--dry-run` | Manual run feels smooth end to end |
| M7 | README, GoReleaser, first tagged release | `go install` works from a clean machine |

## 8. Later (not v0.1)

- `greyprint add redis | db | worker | cron` on an existing project.
- `cmd/worker` option (Greyship queue consumer).
- Project `AGENTS.md` in generated projects describing the layout for coding agents.
- Templates for common extras (CORS, request logging middleware) behind explicit flags.
