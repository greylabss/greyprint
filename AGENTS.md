# AGENTS.md

Instructions for any AI coding agent working on this repo. Read this file and `docs/spec.md` fully before every task.

## What this project is

**Greyprint** is a CLI by GreyLabs that scaffolds a minimal, standard Go backend in seconds. The user picks a router, a database, how they access it, Redis and whether the project should be Greyship-ready. Greyprint writes the files, installs the packages and the project builds and runs immediately.

The promise: *the Go backend you would set up by hand, in five seconds.* Bare minimum, standard layout, nothing clever.

`docs/spec.md` is the source of truth for behaviour, options, generated output and milestones. If a task contradicts it, stop and say so before writing code. If you change behaviour, update the spec in the same change.

## Two kinds of code

This repo contains two kinds of Go code with different rules:

1. **Greyprint's own code** (`cmd/`, `internal/`).
2. **Generated code** (the templates in `internal/templates/`). This is what users read and live with for years. It is the product.

### Rules for generated code (strict)

- **No comments at all.** Not in Go files, not in the Makefile, not in `.env.example`. Names and structure must carry the meaning.
- Reads like a careful senior Go developer wrote it by hand. Boring, obvious, idiomatic.
- Short functions, early returns, clear names. No clever tricks, no reflection, no generics unless unavoidable.
- Only the dependencies the user chose, plus `godotenv`. Use `log/slog`, `os`, `context`, `net/http` and constructors. No DI frameworks, no config libraries, no logging libraries.
- `main.go` is thin: load config, connect, build routes, run, shut down gracefully.
- Every generated project must pass `gofmt`, `go vet` and `go build` with zero changes, for every combination of options. This is tested (see Testing).
- Errors are wrapped with context: `fmt.Errorf("connect database: %w", err)`.
- `context.Context` is the first parameter of anything that does I/O.
- No package-level mutable state.
- Generated file contents must not depend on the order options were chosen or on the machine they were generated on, except the module path and project name.

### Rules for Greyprint's own code

- Same style as above, but comments are allowed **only** to explain *why* when it isn't obvious. Never narrate what the code does. Exported identifiers get short doc comments only where the name isn't enough.
- No commented-out code. No TODOs without an issue reference.
- `gofmt`, `goimports`, `golangci-lint` clean.
- Package names short and lowercase. No `util`, `common`, `helpers`.
- Return errors, never panic. Check with `errors.Is` / `errors.As`.
- Prefer the standard library. Templates use `text/template` + `embed.FS`.
- Never build shell commands by concatenating strings. Use `exec.Command` with separate arguments.

## Dependencies

Approved for Greyprint itself:

| Purpose | Package |
| --- | --- |
| CLI commands and flags | `github.com/spf13/cobra` |
| Interactive prompts | `github.com/charmbracelet/huh` |

Approved for generated projects (only when the user picks them): see the table in `docs/spec.md` §4. Versions are pinned in one place (`internal/versions/versions.go`) and only changed deliberately, with the full test matrix passing.

Adding anything else: explain why the standard library or an approved package isn't enough, and get approval first.

## Repo layout

```
cmd/greyprint/main.go      thin: build the root command and run it
internal/
  cli/                     cobra commands, flags, prompts
  plan/                    options, validation, defaults (pure, no I/O)
  render/                  turns a plan into a file tree (pure, no I/O)
  templates/               embedded templates for generated projects
  versions/                pinned dependency versions
  install/                 go mod tidy / git init
testdata/golden/           expected output trees for key combinations
docs/spec.md
```

`plan` and `render` must stay pure (no disk, no network, no exec) so they are trivially testable. Only `cli` and `install` touch the outside world.

## Testing

- Table-driven tests with the standard `testing` package. Run with `-race`.
- `plan`: every validation rule has a test.
- `render`: golden-file tests. Each golden case is a full expected file tree in `testdata/golden/<case>/`. Update goldens with `go test ./internal/render -update`, then review the diff by eye.
- **Matrix test** (build tag `matrix`): generate every valid combination of options into a temp dir, then run `gofmt -l`, `go vet ./...` and `go build ./...` on each one. Any output or failure fails the test. Also assert that no generated `.go` file contains `//` or `/*`.
- New behaviour comes with tests. Bug fixes come with a test that would have caught the bug.

## Commands

```
make test         go test -race ./...
make matrix       go test -race -tags matrix ./...
make lint         golangci-lint run
make build        build ./bin/greyprint
make golden       regenerate golden files (review the diff before committing)
```

If a target doesn't exist yet and you need it, add it.

## How to work

1. Read `docs/spec.md` sections the task touches.
2. Make the smallest change that does the job. No unrelated refactors.
3. Run `make lint`, `make test` and, if templates changed, `make matrix` before saying you're done.
4. Commit messages in conventional style: `feat(templates): add gin routes`.
5. Finish with a short summary: what changed, whether the spec changed, and anything the founder should check by hand.

## Never

- Add comments to generated code.
- Add a dependency to generated projects that the user didn't choose.
- Ship a template change without the matrix test passing.
- Write secrets or real credentials into generated files. `.env` gets placeholder values only.
