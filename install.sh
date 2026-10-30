#!/bin/sh
set -eu

repo="greylabss/greyprint"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
machine="$(uname -m)"

case "$os" in
	linux|darwin) ;;
	*) printf 'Unsupported operating system: %s\n' "$os" >&2; exit 1 ;;
esac

case "$machine" in
	x86_64|amd64) arch="amd64" ;;
	aarch64|arm64) arch="arm64" ;;
	*) printf 'Unsupported architecture: %s\n' "$machine" >&2; exit 1 ;;
esac

if ! command -v curl >/dev/null 2>&1; then
	printf 'curl is required to install Greyprint.\n' >&2
	exit 1
fi

asset="greyprint_${os}_${arch}.tar.gz"
base="https://github.com/${repo}/releases/latest/download"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

curl -fsSL "${base}/${asset}" -o "${tmp}/${asset}"
curl -fsSL "${base}/checksums.txt" -o "${tmp}/checksums.txt"

expected="$(awk -v file="$asset" '$2 == file {print $1}' "${tmp}/checksums.txt")"
if [ -z "$expected" ]; then
	printf 'No checksum found for %s.\n' "$asset" >&2
	exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
	actual="$(sha256sum "${tmp}/${asset}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
	actual="$(shasum -a 256 "${tmp}/${asset}" | awk '{print $1}')"
else
	printf 'sha256sum or shasum is required to verify the download.\n' >&2
	exit 1
fi
if [ "$actual" != "$expected" ]; then
	printf 'Checksum verification failed for %s.\n' "$asset" >&2
	exit 1
fi

tar -xzf "${tmp}/${asset}" -C "$tmp"
install_dir="${GREYPRINT_INSTALL_DIR:-${HOME}/.local/bin}"
mkdir -p "$install_dir"
install -m 0755 "${tmp}/greyprint" "${install_dir}/greyprint"
printf 'Installed Greyprint to %s/greyprint\n' "$install_dir"
case ":${PATH}:" in
	*":${install_dir}:"*) ;;
	*) printf 'Add this directory to your PATH: %s\n' "$install_dir" ;;
esac
