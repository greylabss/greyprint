package render

import (
	"fmt"
	"go/format"
	"sort"
	"strings"

	"github.com/greylabss/greyprint/internal/plan"
	"github.com/greylabss/greyprint/internal/versions"
)

func Tree(o plan.Options) (map[string][]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	files := map[string]string{
		".env.example":              envFile(o),
		".gitignore":                ".env\n.env.local\n.env.prod\n/bin\n" + ifString(o.Database == "sqlite", "*.db\n", ""),
		"Makefile":                  makefile(o),
		"README.md":                 readme(o),
		"go.mod":                    goMod(o),
		"internal/config/config.go": configFile(o),
		"internal/routes/routes.go": routesFile(o),
		"internal/routes/health.go": healthFile(o),
		"cmd/api/main.go":           mainFile(o),
	}
	if o.Database != "none" {
		files["internal/database/database.go"] = databaseFile(o)
		files["migrations/00001_init.sql"] = migrationFile(o)
		if o.Greyship {
			files["cmd/migrate/main.go"] = migrateFile(o)
		}
		if o.Access == "sqlc" {
			files["sqlc.yaml"] = sqlcConfig(o)
			files["internal/database/queries/items.sql"] = queryFile(o)
			files["internal/database/queries/items.go"] = sqlcGo(o)
		}
	}
	if o.Redis {
		files["internal/cache/cache.go"] = cacheFile(o)
	}
	out := make(map[string][]byte, len(files))
	for path, content := range files {
		if strings.HasSuffix(path, ".go") {
			formatted, err := format.Source([]byte(content))
			if err != nil {
				return nil, fmt.Errorf("format %s: %w", path, err)
			}
			content = string(formatted)
		}
		out[path] = []byte(content)
	}
	return out, nil
}

func Paths(tree map[string][]byte) []string {
	paths := make([]string, 0, len(tree))
	for path := range tree {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func ifString(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func envFile(o plan.Options) string {
	lines := []string{"PORT=8080"}
	if o.Database != "none" {
		lines = append(lines, "DATABASE_URL="+map[string]string{"postgres": "postgres://app:app@localhost:5432/" + o.Name + "?sslmode=disable", "mysql": "app:app@tcp(localhost:3306)/" + o.Name, "sqlite": "file:" + o.Name + ".db"}[o.Database])
	}
	if o.Redis {
		lines = append(lines, "REDIS_URL=localhost:6379")
	}
	return strings.Join(lines, "\n") + "\n"
}

func goMod(o plan.Options) string {
	reqs := []string{"github.com/joho/godotenv " + versions.Godotenv}
	router := map[string]string{"chi": "github.com/go-chi/chi/v5 " + versions.Chi, "gin": "github.com/gin-gonic/gin " + versions.Gin, "echo": "github.com/labstack/echo/v4 " + versions.Echo, "fiber": "github.com/gofiber/fiber/v2 " + versions.Fiber}
	if v := router[o.Router]; v != "" {
		reqs = append(reqs, v)
	}
	if o.Database != "none" {
		switch o.Access {
		case "gorm":
			reqs = append(reqs, "gorm.io/gorm "+versions.GORM)
			switch o.Database {
			case "postgres":
				reqs = append(reqs, "gorm.io/driver/postgres "+versions.GORMPostgres)
			case "mysql":
				reqs = append(reqs, "gorm.io/driver/mysql "+versions.GORMMySQL)
			case "sqlite":
				reqs = append(reqs, "github.com/glebarez/sqlite "+versions.GORMSQLite)
			}
		default:
			switch o.Database {
			case "postgres":
				reqs = append(reqs, "github.com/jackc/pgx/v5 "+versions.PGX)
			case "mysql":
				reqs = append(reqs, "github.com/go-sql-driver/mysql "+versions.MySQL)
			case "sqlite":
				reqs = append(reqs, "modernc.org/sqlite "+versions.SQLite)
			}
		}
		reqs = append(reqs, "github.com/pressly/goose/v3 "+versions.Goose)
	}
	if o.Redis {
		reqs = append(reqs, "github.com/redis/go-redis/v9 "+versions.Redis)
	}
	sort.Strings(reqs)
	return fmt.Sprintf("module %s\n\ngo %s\n\nrequire (\n\t%s\n)\n", o.Module, versions.Go, strings.Join(reqs, "\n\t"))
}

func configFile(o plan.Options) string {
	fields := []string{"Port string"}
	values := []string{"Port: getEnv(\"PORT\", \"8080\")"}
	if o.Database != "none" {
		fields = append(fields, "DatabaseURL string")
		values = append(values, "DatabaseURL: os.Getenv(\"DATABASE_URL\")")
	}
	if o.Redis {
		fields = append(fields, "RedisURL string")
		values = append(values, "RedisURL: getEnv(\"REDIS_URL\", \"localhost:6379\")")
	}
	for i := range values {
		values[i] += ","
	}
	return fmt.Sprintf(`package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	%s
}

func Load() Config {
	_ = godotenv.Load()
	return Config{
		%s
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" { return value }
	return fallback
}
`, strings.Join(fields, "\n\t"), strings.Join(values, "\n\t\t"))
}

func routesFile(o plan.Options) string {
	if o.Router == "fiber" {
		return `package routes

import (
	"context"
	"github.com/gofiber/fiber/v2"
)

func New(dbPing, cachePing func(context.Context) error) *fiber.App {
	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error {
		if dbPing != nil { if err := dbPing(c.UserContext()); err != nil { return c.Status(503).JSON(fiber.Map{"status":"error"}) } }
		if cachePing != nil { if err := cachePing(c.UserContext()); err != nil { return c.Status(503).JSON(fiber.Map{"status":"error"}) } }
		return c.JSON(fiber.Map{"status":"ok"})
	})
	return app
}
`
	}
	imports := []string{"\"context\"", "\"net/http\""}
	var setup, route string
	switch o.Router {
	case "chi":
		imports = append(imports, "\"github.com/go-chi/chi/v5\"")
		setup = "router := chi.NewRouter()"
		route = "router.Get(\"/health\", health(dbPing, cachePing))\n\treturn router"
	case "gin":
		imports = append(imports, "\"github.com/gin-gonic/gin\"")
		setup = "router := gin.New()"
		route = "router.GET(\"/health\", func(c *gin.Context) {\n\t\tif err := check(c.Request.Context(), dbPing, cachePing); err != nil { c.JSON(http.StatusServiceUnavailable, map[string]string{\"status\": \"error\"}); return }\n\t\tc.JSON(http.StatusOK, map[string]string{\"status\": \"ok\"})\n\t})\n\treturn router"
	case "echo":
		imports = append(imports, "\"github.com/labstack/echo/v4\"")
		setup = "router := echo.New()"
		route = "router.GET(\"/health\", func(c echo.Context) error {\n\t\tif err := check(c.Request().Context(), dbPing, cachePing); err != nil { return c.JSON(http.StatusServiceUnavailable, map[string]string{\"status\": \"error\"}) }\n\t\treturn c.JSON(http.StatusOK, map[string]string{\"status\": \"ok\"})\n\t})\n\treturn router"
	default:
		setup = "router := http.NewServeMux()"
		route = "router.HandleFunc(\"GET /health\", health(dbPing, cachePing))\n\treturn router"
	}
	return fmt.Sprintf("package routes\n\nimport (\n\t%s\n)\n\nfunc New(dbPing, cachePing func(context.Context) error) http.Handler {\n\t%s\n\t%s\n}\n", strings.Join(imports, "\n\t"), setup, route)
}

func healthFile(o plan.Options) string {
	if o.Router == "fiber" {
		return "package routes\n"
	}
	if o.Router == "gin" || o.Router == "echo" {
		return `package routes

import "context"

func check(ctx context.Context, dbPing, cachePing func(context.Context) error) error {
	if dbPing != nil { if err := dbPing(ctx); err != nil { return err } }
	if cachePing != nil { if err := cachePing(ctx); err != nil { return err } }
	return nil
}
`
	}
	return `package routes

import (
	"encoding/json"
	"context"
	"net/http"
)

func health(dbPing, cachePing func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if (dbPing != nil && dbPing(r.Context()) != nil) || (cachePing != nil && cachePing(r.Context()) != nil) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status":"error"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status":"ok"})
	}
}
`
}

func mainFile(o plan.Options) string {
	imports := []string{"\"context\"", "\"errors\"", "\"log/slog\"", "\"os\"", "\"os/signal\"", "\"syscall\"", "\"time\"", fmt.Sprintf("%q", o.Module+"/internal/config"), fmt.Sprintf("%q", o.Module+"/internal/routes")}
	if o.Router == "fiber" {
		imports = []string{"\"context\"", "\"log/slog\"", "\"os\"", "\"os/signal\"", "\"syscall\"", "\"time\"", fmt.Sprintf("%q", o.Module+"/internal/config"), fmt.Sprintf("%q", o.Module+"/internal/routes")}
	}
	if o.Router != "fiber" {
		imports = append(imports, "\"net/http\"")
	}
	if o.Database != "none" {
		imports = append(imports, fmt.Sprintf("%q", o.Module+"/internal/database"))
	}
	if o.Redis {
		imports = append(imports, fmt.Sprintf("%q", o.Module+"/internal/cache"))
	}
	setup := []string{"cfg := config.Load()"}
	pingers := []string{"nil", "nil"}
	if o.Database != "none" {
		setup = append(setup, "dbPing, dbClose, err := database.Connect(ctx, cfg.DatabaseURL)\n\tif err != nil { return err }\n\tdefer dbClose()")
		pingers[0] = "dbPing"
	}
	if o.Redis {
		setup = append(setup, "cachePing, cacheClose, err := cache.Connect(ctx, cfg.RedisURL)\n\tif err != nil { return err }\n\tdefer cacheClose()")
		pingers[1] = "cachePing"
	}
	if o.Router == "fiber" {
		setup = append(setup, "app := routes.New("+strings.Join(pingers, ", ")+")\n\tgo func() { <-ctx.Done(); shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel(); _ = app.ShutdownWithContext(shutdownCtx) }()\n\tslog.Info(\"server started\", \"port\", cfg.Port)\n\tif err := app.Listen(\":\" + cfg.Port); err != nil && ctx.Err() == nil { return err }\n\treturn nil")
	} else {
		setup = append(setup, "server := &http.Server{Addr: \":\" + cfg.Port, Handler: routes.New("+strings.Join(pingers, ", ")+"), ReadHeaderTimeout: 5 * time.Second}\n\tgo func() { <-ctx.Done(); shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel(); _ = server.Shutdown(shutdownCtx) }()\n\tslog.Info(\"server started\", \"port\", cfg.Port)\n\tif err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) { return err }\n\treturn nil")
	}
	return fmt.Sprintf("package main\n\nimport (\n\t%s\n)\n\nfunc main() {\n\tif err := run(); err != nil { slog.Error(\"server stopped\", \"error\", err); os.Exit(1) }\n}\n\nfunc run() error {\n\tctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)\n\tdefer stop()\n\t%s\n}\n", strings.Join(imports, "\n\t"), strings.Join(setup, "\n\t"))
}

func databaseFile(o plan.Options) string {
	driver := map[string]string{"postgres": "pgx", "mysql": "mysql", "sqlite": "sqlite"}[o.Database]
	imp := map[string]string{"postgres": "_ \"github.com/jackc/pgx/v5/stdlib\"", "mysql": "_ \"github.com/go-sql-driver/mysql\"", "sqlite": "_ \"modernc.org/sqlite\""}[o.Database]
	if o.Access == "gorm" {
		gormDriver := map[string]string{"postgres": "gorm.io/driver/postgres", "mysql": "gorm.io/driver/mysql", "sqlite": "github.com/glebarez/sqlite"}[o.Database]
		return fmt.Sprintf("package database\n\nimport (\n\t\"context\"\n\t\"fmt\"\n\t%q\n\t\"gorm.io/gorm\"\n)\n\nfunc Connect(ctx context.Context, url string) (func(context.Context) error, func(), error) {\n\tdb, err := gorm.Open(%s.Open(url), &gorm.Config{})\n\tif err != nil { return nil, nil, fmt.Errorf(\"connect database: %%w\", err) }\n\tsqlDB, err := db.DB()\n\tif err != nil { return nil, nil, fmt.Errorf(\"get database connection: %%w\", err) }\n\tif err := sqlDB.PingContext(ctx); err != nil { _ = sqlDB.Close(); return nil, nil, fmt.Errorf(\"ping database: %%w\", err) }\n\treturn func(ctx context.Context) error { return sqlDB.PingContext(ctx) }, func() { _ = sqlDB.Close() }, nil\n}\n", gormDriver, map[string]string{"postgres": "postgres", "mysql": "mysql", "sqlite": "sqlite"}[o.Database])
	}
	return fmt.Sprintf("package database\n\nimport (\n\t\"context\"\n\t\"database/sql\"\n\t\"fmt\"\n\t%s\n)\n\nfunc Connect(ctx context.Context, url string) (func(context.Context) error, func(), error) {\n\tdb, err := sql.Open(%q, url)\n\tif err != nil { return nil, nil, fmt.Errorf(\"open database: %%w\", err) }\n\tif err := db.PingContext(ctx); err != nil { _ = db.Close(); return nil, nil, fmt.Errorf(\"ping database: %%w\", err) }\n\treturn func(ctx context.Context) error { return db.PingContext(ctx) }, func() { _ = db.Close() }, nil\n}\n", imp, driver)
}

func cacheFile(o plan.Options) string {
	return `package cache

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, address string) (func(context.Context) error, func(), error) {
	client := redis.NewClient(&redis.Options{Addr: address})
	if err := client.Ping(ctx).Err(); err != nil { _ = client.Close(); return nil, nil, fmt.Errorf("connect redis: %w", err) }
	return func(ctx context.Context) error { return client.Ping(ctx).Err() }, func() { _ = client.Close() }, nil
}
`
}

func migrationFile(o plan.Options) string {
	if o.Database == "mysql" {
		return "-- +goose Up\nCREATE TABLE items (\n    id BIGINT AUTO_INCREMENT PRIMARY KEY,\n    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP\n);\n\n-- +goose Down\nDROP TABLE items;\n"
	}
	if o.Database == "sqlite" {
		return "-- +goose Up\nCREATE TABLE items (\n    id INTEGER PRIMARY KEY AUTOINCREMENT,\n    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP\n);\n\n-- +goose Down\nDROP TABLE items;\n"
	}
	return "-- +goose Up\nCREATE TABLE items (\n    id BIGSERIAL PRIMARY KEY,\n    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()\n);\n\n-- +goose Down\nDROP TABLE items;\n"
}

func migrateFile(o plan.Options) string {
	imp := map[string]string{"postgres": "_ \"github.com/jackc/pgx/v5/stdlib\"", "mysql": "_ \"github.com/go-sql-driver/mysql\"", "sqlite": "_ \"modernc.org/sqlite\""}[o.Database]
	driver := map[string]string{"postgres": "pgx", "mysql": "mysql", "sqlite": "sqlite"}[o.Database]
	return fmt.Sprintf("package main\n\nimport (\n\t\"database/sql\"\n\t\"log/slog\"\n\t\"os\"\n\t\"github.com/pressly/goose/v3\"\n\t\"github.com/joho/godotenv\"\n\t%s\n)\n\nfunc main() {\n\t_ = godotenv.Load()\n\tdb, err := sql.Open(%q, os.Getenv(\"DATABASE_URL\"))\n\tif err != nil { slog.Error(\"open database\", \"error\", err); os.Exit(1) }\n\tdefer db.Close()\n\tif err := goose.Up(db, \"migrations\"); err != nil { slog.Error(\"run migrations\", \"error\", err); os.Exit(1) }\n}\n", imp, driver)
}

func sqlcConfig(o plan.Options) string {
	return "version: \"2\"\nsql:\n  - engine: \"" + map[string]string{"postgres": "postgresql", "mysql": "mysql", "sqlite": "sqlite"}[o.Database] + "\"\n    queries: \"internal/database/queries\"\n    schema: \"migrations\"\n    gen:\n      go:\n        package: \"queries\"\n        out: \"internal/database/queries\"\n"
}
func queryFile(o plan.Options) string {
	return "-- name: ListItems :many\nSELECT id, created_at FROM items ORDER BY id;\n"
}
func sqlcGo(o plan.Options) string {
	return `package queries

import (
	"context"
	"database/sql"
)

type Item struct { ID int64; CreatedAt string }
type Queries struct { db *sql.DB }
func New(db *sql.DB) *Queries { return &Queries{db: db} }
func (q *Queries) ListItems(ctx context.Context) ([]Item, error) {
	rows, err := q.db.QueryContext(ctx, "SELECT id, created_at FROM items ORDER BY id")
	if err != nil { return nil, err }
	defer rows.Close()
	items := []Item{}
	for rows.Next() { var item Item; if err := rows.Scan(&item.ID, &item.CreatedAt); err != nil { return nil, err }; items = append(items, item) }
	return items, rows.Err()
}
`
}

func makefile(o plan.Options) string {
	lines := []string{".PHONY: run build test tidy", "", "run:", "\tgo run ./cmd/api", "", "build:", "\tgo build ./cmd/api", "", "test:", "\tgo test ./...", "", "tidy:", "\tgo mod tidy"}
	if o.Database != "none" {
		lines = append(lines, "", "migrate:")
		if o.Greyship {
			lines = append(lines, "\tgo run ./cmd/migrate")
		} else {
			lines = append(lines, "\tgo run github.com/pressly/goose/v3/cmd/goose -dir migrations "+map[string]string{"postgres": "postgres", "mysql": "mysql", "sqlite": "sqlite3"}[o.Database]+" \"$(DATABASE_URL)\" up")
		}
	}
	if o.Access == "sqlc" {
		lines = append(lines, "", "sqlc:", "\tsqlc generate")
	}
	return strings.Join(lines, "\n") + "\n"
}

func readme(o plan.Options) string {
	text := fmt.Sprintf("# %s\n\nA Go API generated by Greyprint.\n\n## Run\n\n```sh\ncp .env.example .env\nmake run\n```\n\n## Layout\n\n- `cmd/api`: server entry point\n- `internal/config`: environment configuration\n- `internal/routes`: HTTP routes and health check\n", o.Name)
	if o.Database != "none" {
		text += "- `internal/database`: database connection\n- `migrations`: database schema\n"
	}
	if o.Redis {
		text += "- `internal/cache`: Redis connection\n"
	}
	if o.Greyship {
		text += "\n## Deploy\n\n```sh\ngrey deploy\n```\n"
	}
	return text
}
