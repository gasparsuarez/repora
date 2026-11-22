#!/usr/bin/env sh
#
# Install or update Repora from GitHub releases.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/gasparsuarez/repora/main/scripts/install.sh | sh
#   ./scripts/install.sh --version v0.1.3 --dir ~/.local/bin
#
# Environment:
#   REPORA_VERSION       Install a specific tag (default: latest release).
#   REPORA_INSTALL_DIR   Install directory (default: ~/.local/bin).
#   REPORA_FORCE         Set to 1 to replace a Homebrew-managed binary.

set -eu

OWNER="gasparsuarez"
REPO="repora"
BINARY="repora"

version="${REPORA_VERSION:-}"
install_dir="${REPORA_INSTALL_DIR:-${HOME}/.local/bin}"
force="${REPORA_FORCE:-0}"

usage() {
	cat <<EOF
Install or update Repora.

Usage:
  install.sh [--version TAG] [--dir DIR] [--force]

Options:
  --version TAG   Install a specific release (for example v0.1.3).
  --dir DIR       Install directory (default: ~/.local/bin).
  --force         Replace a Homebrew-managed binary.
  -h, --help      Show this help.

Environment:
  REPORA_VERSION       Same as --version.
  REPORA_INSTALL_DIR   Same as --dir.
  REPORA_FORCE         Same as --force.
EOF
}

fail() {
	printf 'repora install: %s\n' "$*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

while [ "$#" -gt 0 ]; do
	case "$1" in
	--version)
		[ "$#" -ge 2 ] || fail "--version needs a tag"
		version="$2"
		shift 2
		;;
	--dir)
		[ "$#" -ge 2 ] || fail "--dir needs a path"
		install_dir="$2"
		shift 2
		;;
	--force)
		force=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		fail "unknown argument: $1"
		;;
	esac
done

need curl
need tar
need uname
need mktemp

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
darwin | linux) ;;
*) fail "unsupported operating system: $os" ;;
esac

machine="$(uname -m)"
case "$machine" in
x86_64 | amd64) arch="amd64" ;;
arm64 | aarch64) arch="arm64" ;;
*) fail "unsupported architecture: $machine" ;;
esac

if [ -z "$version" ]; then
	latest_url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
		"https://github.com/${OWNER}/${REPO}/releases/latest")" ||
		fail "could not resolve the latest release"
	case "$latest_url" in
	*/releases/tag/*) version="${latest_url##*/}" ;;
	*) fail "could not resolve the latest release (got: $latest_url)" ;;
	esac
fi

case "$version" in
v*) tag="$version" ;;
*) tag="v$version" ;;
esac
version_number="${tag#v}"

if [ "$force" != "1" ] && command -v "$BINARY" >/dev/null 2>&1; then
	current="$(command -v "$BINARY")"
	case "$current" in
	*/Cellar/* | */homebrew/*)
		fail "Repora is managed by Homebrew (${current}). Run: brew upgrade repora"
		;;
	esac
fi

asset="${BINARY}_${version_number}_${os}_${arch}.tar.gz"
release_url="https://github.com/${OWNER}/${REPO}/releases/download/${tag}"

tmp="$(mktemp -d "${TMPDIR:-/tmp}/repora-install.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT INT TERM

printf 'Downloading Repora %s (%s/%s)...\n' "$tag" "$os" "$arch"
curl -fsSL "${release_url}/${asset}" -o "${tmp}/${asset}" ||
	fail "download failed: ${release_url}/${asset}"
curl -fsSL "${release_url}/SHA256SUMS" -o "${tmp}/SHA256SUMS" ||
	fail "could not download SHA256SUMS"

checksum_line="$(awk -v file="$asset" '$2 == file { print; exit }' "${tmp}/SHA256SUMS")"
[ -n "$checksum_line" ] || fail "checksum for ${asset} not found"
printf '%s\n' "$checksum_line" >"${tmp}/${asset}.sha256"

if command -v sha256sum >/dev/null 2>&1; then
	(cd "$tmp" && sha256sum -c "${asset}.sha256") ||
		fail "checksum verification failed"
elif command -v shasum >/dev/null 2>&1; then
	(cd "$tmp" && shasum -a 256 -c "${asset}.sha256") ||
		fail "checksum verification failed"
else
	fail "need sha256sum or shasum to verify the download"
fi

tar -xzf "${tmp}/${asset}" -C "$tmp" "$BINARY" ||
	fail "could not extract ${asset}"

mkdir -p "$install_dir"
if command -v install >/dev/null 2>&1; then
	install -m 0755 "${tmp}/${BINARY}" "${install_dir}/${BINARY}"
else
	cp "${tmp}/${BINARY}" "${install_dir}/${BINARY}"
	chmod 0755 "${install_dir}/${BINARY}"
fi

printf 'Installed %s\n' "${install_dir}/${BINARY}"
"${install_dir}/${BINARY}" --version || true

case ":${PATH}:" in
*":${install_dir}:"*) ;;
*)
	printf '\nAdd %s to PATH:\n  export PATH="%s:$PATH"\n' \
		"$install_dir" "$install_dir"
	;;
esac
