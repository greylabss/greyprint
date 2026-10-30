# Contributing to Greyprint

Thanks for helping improve Greyprint. Bug reports, documentation fixes, and focused code changes are welcome.

## Before opening an issue

Search existing issues first. Include the Greyprint version, operating system, Go version, command and flags used, and the complete error output. Do not include secrets or private connection strings.

## Before opening a pull request

1. Open or reference an issue for a substantial behavior change.
2. Keep changes focused and explain the user-facing effect in the pull request description.
3. Update `docs/spec.md` whenever behavior, options, or generated output changes.
4. Add or update tests. Template changes require regenerated golden files and the full matrix check.
5. Run the checks below before requesting review.

```sh
make lint
make test
make matrix
make build
```

Generated projects are the product. Keep them small, idiomatic, and free of Go comments. Do not add dependencies that the selected options do not need. See [AGENTS.md](AGENTS.md) and [docs/spec.md](docs/spec.md) for the detailed engineering rules.

## Pull request checklist

- [ ] The change matches or updates the specification.
- [ ] Tests cover the behavior and any regression.
- [ ] `make lint`, `make test`, `make matrix`, and `make build` pass as applicable.
- [ ] Generated output contains no credentials or machine-specific values.
- [ ] User documentation is updated when needed.

## Release process

Releases are published from version tags. Maintainers create and push a tag such as `v0.1.0`; the GitHub Actions release workflow builds archives for Linux, macOS, and Windows on amd64 and arm64, produces SHA-256 checksums, and publishes a GitHub Release. Do not publish a release manually from a developer machine.

## Community standards

Participation follows the [Code of Conduct](CODE_OF_CONDUCT.md). Report unacceptable behavior privately to the maintainers.
