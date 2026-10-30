# Greyprint

Greyprint creates a small Go HTTP backend from a single command. Choose a router, an optional database and access style, Redis, and Greyship support. The generated project includes a health endpoint, configuration loading, build and run targets, and only the files needed by those choices.

## Install

Install the latest release on macOS or Linux with the repository script:

```sh
curl -fsSL https://raw.githubusercontent.com/greylabss/greyprint/main/install.sh | sh
```

On Windows, run this in PowerShell:

```powershell
irm https://raw.githubusercontent.com/greylabss/greyprint/main/install.ps1 | iex
```

Both scripts select the matching release archive and verify its SHA-256 checksum before installation. The Unix script installs to `~/.local/bin` by default; set `GREYPRINT_INSTALL_DIR` to choose another directory. The Windows script installs under the current user's programs directory and adds it to the user `PATH`.

You can also install the latest tagged version with Go:

```sh
go install github.com/greylabss/greyprint/cmd/greyprint@latest
```

The executable is named `greyprint`. Until the first release is published, clone the repository and run `make build` to build it from source.

## Create a project

Run the interactive setup:

```sh
greyprint new storefront
```

Prompts cover the project name (when omitted), module path, router, database, database access, Redis, and Greyship readiness. Flags supplied on the command line skip their matching prompts. The final summary asks for confirmation.

For non-interactive use, pass `--yes` to accept defaults for unspecified choices:

```sh
greyprint new storefront --yes
```

Or define the full configuration explicitly:

```sh
greyprint new storefront \
  --module github.com/acme/storefront \
  --router chi \
  --db postgres \
  --access sqlc \
  --redis \
  --greyship \
  --dir ./storefront
```

`--json` disables prompts, applies defaults, and prints a JSON result containing the project name, directory, module, options, generated paths, and install-step results. `--dry-run` prints the paths that would be generated without writing files. The output path can be new or an existing empty directory.

## Options

| Flag | Values | Default | Effect |
| --- | --- | --- | --- |
| `--module` | Go module path | `github.com/<git user.name or me>/<name>` | Sets the module declaration and generated imports. |
| `--router` | `stdlib`, `chi`, `gin`, `echo`, `fiber` | `stdlib` | Selects the HTTP router. Fiber uses its native server. |
| `--db` | `none`, `postgres`, `mysql`, `sqlite` | `none` | Adds database connection and migration files. |
| `--access` | `driver`, `sqlc`, `gorm` | `driver` | Selects direct SQL, generated query helpers, or GORM. Ignored with no database. |
| `--redis` | boolean | `false` | Adds Redis configuration, connection, and health checks. |
| `--greyship` | boolean | `true` | Adds the `cmd/migrate` release command and deploy hint when a database is selected. |
| `--dir` | path | `./<name>` | Sets the output directory. |
| `--yes`, `-y` | boolean | `false` | Suppresses prompts and accepts defaults. |
| `--no-install` | boolean | `false` | Skips `go mod tidy`. |
| `--no-git` | boolean | `false` | Skips `git init`. |
| `--dry-run` | boolean | `false` | Lists generated paths without writing. |
| `--json` | boolean | `false` | Uses defaults without prompts and emits JSON. |

Project names use lowercase letters, digits, hyphens, and underscores; they start with a letter and contain at most 64 characters. Database access modes other than `driver` require a database.

## First run

After creation, Greyprint prints the next commands. In the project directory:

```sh
cp .env.example .env
make run
```

The API listens on `PORT` (default `8080`). `GET /health` returns `{"status":"ok"}` when configured dependencies are healthy, and HTTP 503 when a database or Redis health check fails. The server handles SIGINT and SIGTERM and allows up to ten seconds for graceful shutdown.

The sample environment contains local connection defaults, not credentials for a hosted service. Update the values to match your local services. For SQLite, the database file is created in the project directory and is ignored by Git.

## Generated layout

```text
project/
  cmd/
    api/main.go
    migrate/main.go          when a database and Greyship are selected
  internal/
    config/config.go
    routes/routes.go
    routes/health.go
    database/database.go     when a database is selected
    database/queries/        when sqlc is selected
    cache/cache.go           when Redis is selected
  migrations/                when a database is selected
  sqlc.yaml                  when sqlc is selected
  .env.example
  .gitignore
  Makefile
  README.md
  go.mod
```

The `config` package loads `.env` when present and reads environment variables. The `routes` package owns the health route. The `database` and `cache` packages connect and expose health checks, without adding service or repository layers. `cmd/api` wires these pieces together and runs the HTTP server.

## Database and query workflows

The default `driver` access style uses `database/sql` with the selected driver. GORM uses its matching dialect. `sqlc` includes a query and generated Go query helper so the project can compile without sqlc installed; `make sqlc` regenerates the helper after sqlc is installed.

Every selected database receives a goose migration that creates a small `items` table. Run it with `make migrate`. With Greyship enabled, this invokes `cmd/migrate`, which is suitable as a release command. Without Greyship, the Makefile invokes goose directly.

The Makefile also provides `run`, `build`, `test`, and `tidy` targets. Generated modules pin their direct dependencies so `go mod tidy` can resolve them consistently.

## Greyship

Greyship support is enabled by default. The generated layout provides `cmd/api` as the web service and `cmd/migrate` as the release command when a database is present. The server uses the injected `PORT` environment variable. No Greyship-specific configuration file is generated.

Deploy from the generated project with:

```sh
grey deploy
```

Turn this support off with `--greyship=false`. This removes the generated migrate command and deploy instructions while keeping the database migration target available.

## Troubleshooting

- If dependency installation fails after generation, the project files remain on disk. Run `go mod tidy` from the project directory and inspect the reported step error.
- If `make run` cannot connect to a selected database or Redis instance, check `.env` and start the local service first.
- If the output directory contains files, choose another `--dir` or empty that directory before generating.
- Use `greyprint version` to check the installed CLI version.

## For contributors

The contributor workflow, generated-code rules, architecture, and option matrix live in [the project specification](spec.md). Validate changes with `make lint`, `make test`, and `make matrix` after template changes.
