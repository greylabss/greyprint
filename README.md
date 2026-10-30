# Greyprint

Greyprint is a small, open-source CLI from GreyLabs that scaffolds standard Go backends. Pick a router, optional database and access style, Redis, and Greyship support; Greyprint creates a runnable project with only the pieces you selected.

## Install

### macOS and Linux

The install script downloads the latest release for your operating system and architecture, verifies its SHA-256 checksum, and installs to `~/.local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/greylabss/greyprint/main/install.sh | sh
```

Set `GREYPRINT_INSTALL_DIR` to install elsewhere. Add the install directory to `PATH` if it is not there already.

### Windows PowerShell

Run this in PowerShell. The installer downloads and verifies the latest Windows binary, installs it under your user profile, and adds that directory to your user `PATH`:

```powershell
irm https://raw.githubusercontent.com/greylabss/greyprint/main/install.ps1 | iex
```

Set `$env:GREYPRINT_INSTALL_DIR` before running the command to choose another destination. Open a new terminal after installation so the updated `PATH` is picked up.

### Install with Go

```sh
go install github.com/greylabss/greyprint/cmd/greyprint@latest
```

This installs the latest tagged version. To build from source, clone this repository and run `make build`.

## Use

Start the interactive setup:

```sh
greyprint new storefront
```

Create a project non-interactively with defaults:

```sh
greyprint new storefront --yes
```

Choose options with flags:

```sh
greyprint new storefront \
  --module github.com/acme/storefront \
  --router chi \
  --db postgres \
  --access sqlc \
  --redis \
  --greyship
```

Then enter the project and run it:

```sh
cd storefront
cp .env.example .env
make run
```

The generated API listens on `PORT` (default `8080`) and provides `GET /health`. See the [full user guide](docs/guide.md) for every option and the generated project layout.

## Development

```sh
make test
make matrix
make lint
make build
```

See [CONTRIBUTING.md](CONTRIBUTING.md) before opening an issue or pull request. Greyprint is released from version tags; the [release workflow](.github/workflows/release.yml) builds and publishes checksummed binaries for Linux, macOS, and Windows on amd64 and arm64.

## License

Greyprint is available under the [MIT License](LICENSE).
