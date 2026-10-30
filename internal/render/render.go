package render

import (
	"fmt"
	"github.com/greylabss/greyprint/internal/templates"
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
	moduleFile, err := goMod(o)
	if err != nil {
		return nil, err
	}
	config, err := configFile(o)
	if err != nil {
		return nil, err
	}
	env, err := envFile(o)
	if err != nil {
		return nil, err
	}
	files := map[string]string{
		".env.example":              env,
		".gitignore":                ".env\n.env.local\n.env.prod\n/bin\n" + ifString(o.Database == "sqlite", "*.db\n", ""),
		"Makefile":                  makefile(o),
		"README.md":                 readme(o),
		"go.mod":                    moduleFile,
		"internal/config/config.go": config,
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

func envFile(o plan.Options) (string, error) {
	lines := []string{"PORT=8080"}
	if o.Database != "none" {
		lines = append(lines, "DATABASE_URL="+map[string]string{"postgres": "postgres://app:app@localhost:5432/" + o.Name + "?sslmode=disable", "mysql": "app:app@tcp(localhost:3306)/" + o.Name, "sqlite": "file:" + o.Name + ".db"}[o.Database])
	}
	if o.Redis {
		lines = append(lines, "REDIS_URL=localhost:6379")
	}
	content, err := templates.Render("env.tmpl", lines)
	return string(content), err
}

func goMod(o plan.Options) (string, error) {
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
	content, err := templates.Render("gomod.tmpl", struct {
		Module       string
		GoVersion    string
		Requirements []string
	}{Module: o.Module, GoVersion: versions.Go, Requirements: reqs})
	return string(content), err
}

func configFile(o plan.Options) (string, error) {
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
	content, err := templates.Render("config.tmpl", struct {
		Fields []string
		Values []string
	}{Fields: fields, Values: values})
	return string(content), err
}

func routesFile(o plan.Options) string {
	imports := []string{}
	params, args := routeClients(o)
	var setup, route, resultType string
	switch o.Router {
	case "chi":
		imports = append(imports, "\"github.com/go-chi/chi/v5\"")
		setup = "router := chi.NewRouter()"
		resultType = "http.Handler"
		route = "router.Get(\"/health\", health(" + strings.Join(args, ", ") + "))\n\treturn router"
	case "gin":
		imports = append(imports, "\"github.com/gin-gonic/gin\"")
		setup = "router := gin.New()"
		resultType = "http.Handler"
		route = "router.GET(\"/health\", func(c *gin.Context) {\n\t\tif err := check(c.Request.Context(), " + strings.Join(args, ", ") + "); err != nil { c.JSON(http.StatusServiceUnavailable, map[string]string{\"status\": \"error\"}); return }\n\t\tc.JSON(http.StatusOK, map[string]string{\"status\": \"ok\"})\n\t})\n\treturn router"
	case "echo":
		imports = append(imports, "\"github.com/labstack/echo/v4\"")
		setup = "router := echo.New()"
		resultType = "http.Handler"
		route = "router.GET(\"/health\", func(c echo.Context) error {\n\t\tif err := check(c.Request().Context(), " + strings.Join(args, ", ") + "); err != nil { return c.JSON(http.StatusServiceUnavailable, map[string]string{\"status\": \"error\"}) }\n\t\treturn c.JSON(http.StatusOK, map[string]string{\"status\": \"ok\"})\n\t})\n\treturn router"
	case "fiber":
		imports = append(imports, "\"github.com/gofiber/fiber/v2\"")
		setup = "app := fiber.New()"
		resultType = "*fiber.App"
		route = "app.Get(\"/health\", func(c *fiber.Ctx) error {\n\t\tif err := check(c.UserContext(), " + strings.Join(args, ", ") + "); err != nil { return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{\"status\": \"error\"}) }\n\t\treturn c.JSON(fiber.Map{\"status\": \"ok\"})\n\t})\n\treturn app"
	default:
		setup = "router := http.NewServeMux()"
		resultType = "http.Handler"
		route = "router.HandleFunc(\"GET /health\", health(" + strings.Join(args, ", ") + "))\n\treturn router"
	}
	imports = append(imports, routeClientImports(o)...)
	imports = append(imports, "\"net/http\"")
	return fmt.Sprintf("package routes\n\nimport (\n\t%s\n)\n\nfunc New(%s) %s {\n\t%s\n\t%s\n}\n", strings.Join(imports, "\n\t"), strings.Join(params, ", "), resultType, setup, route)
}

func routeClients(o plan.Options) ([]string, []string) {
	params := []string{}
	args := []string{}
	if o.Database != "none" {
		if o.Access == "gorm" {
			params = append(params, "db *gorm.DB")
		} else {
			params = append(params, "db *sql.DB")
		}
		args = append(args, "db")
	}
	if o.Redis {
		params = append(params, "redisClient *redis.Client")
		args = append(args, "redisClient")
	}
	return params, args
}

func routeClientDBImport(o plan.Options) string {
	if o.Access == "gorm" {
		return "\"gorm.io/gorm\""
	}
	return "\"database/sql\""
}

func routeClientImports(o plan.Options) []string {
	imports := []string{}
	if o.Database != "none" {
		imports = append(imports, routeClientDBImport(o))
	}
	if o.Redis {
		imports = append(imports, "\"github.com/redis/go-redis/v9\"")
	}
	return imports
}

func healthFile(o plan.Options) string {
	imports := []string{"\"context\""}
	params, args := routeClients(o)
	var checks strings.Builder
	if o.Database != "none" {
		imports = append(imports, routeClientDBImport(o))
		if o.Access == "gorm" {
			checks.WriteString("\tif db != nil { sqlDB, err := db.DB(); if err != nil { return err }; if err := sqlDB.PingContext(ctx); err != nil { return err } }\n")
		} else {
			checks.WriteString("\tif db != nil { if err := db.PingContext(ctx); err != nil { return err } }\n")
		}
	}
	if o.Redis {
		imports = append(imports, "\"github.com/redis/go-redis/v9\"")
		checks.WriteString("\tif redisClient != nil { if err := redisClient.Ping(ctx).Err(); err != nil { return err } }\n")
	}
	if o.Router == "stdlib" || o.Router == "chi" {
		imports = append(imports, "\"encoding/json\"", "\"net/http\"")
	}
	check := fmt.Sprintf("func check(ctx context.Context, %s) error {\n%s\treturn nil\n}\n", strings.Join(params, ", "), checks.String())
	if len(params) == 0 {
		check = "func check(ctx context.Context) error {\n\treturn nil\n}\n"
	}
	if o.Router == "gin" || o.Router == "echo" || o.Router == "fiber" {
		return fmt.Sprintf("package routes\n\nimport (\n\t%s\n)\n\n%s", strings.Join(imports, "\n\t"), check)
	}
	return fmt.Sprintf(`package routes

import (
	%s
)

%s
func health(%s) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := check(r.Context(), %s); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status":"error"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status":"ok"})
	}
}
`, strings.Join(imports, "\n\t"), check, strings.Join(params, ", "), strings.Join(args, ", "))
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
	routeArgs := []string{}
	if o.Database != "none" {
		setup = append(setup, "db, err := database.Connect(ctx, cfg.DatabaseURL)\n\tif err != nil { return err }")
		if o.Access == "gorm" {
			setup = append(setup, "sqlDB, err := db.DB()\n\tif err != nil { return fmt.Errorf(\"get database connection: %w\", err) }\n\tdefer sqlDB.Close()")
			imports = append(imports, "\"fmt\"")
		} else {
			setup = append(setup, "defer db.Close()")
		}
		routeArgs = append(routeArgs, "db")
	}
	if o.Redis {
		setup = append(setup, "redisClient, err := cache.Connect(ctx, cfg.RedisURL)\n\tif err != nil { return err }\n\tdefer redisClient.Close()")
		routeArgs = append(routeArgs, "redisClient")
	}
	router := "routes.New(" + strings.Join(routeArgs, ", ") + ")"
	if o.Router == "fiber" {
		setup = append(setup, "app := "+router+"\n\tgo func() { <-ctx.Done(); shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel(); _ = app.ShutdownWithContext(shutdownCtx) }()\n\tslog.Info(\"server started\", \"port\", cfg.Port)\n\tif err := app.Listen(\":\" + cfg.Port); err != nil && ctx.Err() == nil { return err }\n\treturn nil")
	} else {
		setup = append(setup, "server := &http.Server{Addr: \":\" + cfg.Port, Handler: "+router+", ReadHeaderTimeout: 5 * time.Second}\n\tgo func() { <-ctx.Done(); shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel(); _ = server.Shutdown(shutdownCtx) }()\n\tslog.Info(\"server started\", \"port\", cfg.Port)\n\tif err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) { return err }\n\treturn nil")
	}
	return fmt.Sprintf("package main\n\nimport (\n\t%s\n)\n\nfunc main() {\n\tif err := run(); err != nil { slog.Error(\"server stopped\", \"error\", err); os.Exit(1) }\n}\n\nfunc run() error {\n\tctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)\n\tdefer stop()\n\t%s\n}\n", strings.Join(imports, "\n\t"), strings.Join(setup, "\n\t"))
}

func databaseFile(o plan.Options) string {
	driver := map[string]string{"postgres": "pgx", "mysql": "mysql", "sqlite": "sqlite"}[o.Database]
	imp := map[string]string{"postgres": "_ \"github.com/jackc/pgx/v5/stdlib\"", "mysql": "_ \"github.com/go-sql-driver/mysql\"", "sqlite": "_ \"modernc.org/sqlite\""}[o.Database]
	if o.Access == "gorm" {
		gormDriver := map[string]string{"postgres": "gorm.io/driver/postgres", "mysql": "gorm.io/driver/mysql", "sqlite": "github.com/glebarez/sqlite"}[o.Database]
		return fmt.Sprintf("package database\n\nimport (\n\t\"context\"\n\t\"fmt\"\n\t%q\n\t\"gorm.io/gorm\"\n)\n\nfunc Connect(ctx context.Context, url string) (*gorm.DB, error) {\n\tdb, err := gorm.Open(%s.Open(url), &gorm.Config{})\n\tif err != nil { return nil, fmt.Errorf(\"connect database: %%w\", err) }\n\tsqlDB, err := db.DB()\n\tif err != nil { return nil, fmt.Errorf(\"get database connection: %%w\", err) }\n\tif err := sqlDB.PingContext(ctx); err != nil { _ = sqlDB.Close(); return nil, fmt.Errorf(\"ping database: %%w\", err) }\n\treturn db, nil\n}\n", gormDriver, map[string]string{"postgres": "postgres", "mysql": "mysql", "sqlite": "sqlite"}[o.Database])
	}
	return fmt.Sprintf("package database\n\nimport (\n\t\"context\"\n\t\"database/sql\"\n\t\"fmt\"\n\t%s\n)\n\nfunc Connect(ctx context.Context, url string) (*sql.DB, error) {\n\tdb, err := sql.Open(%q, url)\n\tif err != nil { return nil, fmt.Errorf(\"open database: %%w\", err) }\n\tif err := db.PingContext(ctx); err != nil { _ = db.Close(); return nil, fmt.Errorf(\"ping database: %%w\", err) }\n\treturn db, nil\n}\n", imp, driver)
}

func cacheFile(o plan.Options) string {
	return `package cache

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, address string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{Addr: address})
	if err := client.Ping(ctx).Err(); err != nil { _ = client.Close(); return nil, fmt.Errorf("connect redis: %w", err) }
	return client, nil
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
	return fmt.Sprintf("package main\n\nimport (\n\t\"context\"\n\t\"database/sql\"\n\t\"log/slog\"\n\t\"os\"\n\t\"github.com/pressly/goose/v3\"\n\t\"github.com/joho/godotenv\"\n\t%s\n)\n\nfunc main() {\n\t_ = godotenv.Load()\n\tdb, err := sql.Open(%q, os.Getenv(\"DATABASE_URL\"))\n\tif err != nil { slog.Error(\"open database\", \"error\", err); os.Exit(1) }\n\tdefer db.Close()\n\tif err := goose.UpContext(context.Background(), db, \"migrations\"); err != nil { slog.Error(\"run migrations\", \"error\", err); os.Exit(1) }\n}\n", imp, driver)
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
