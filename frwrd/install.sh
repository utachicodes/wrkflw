#!/usr/bin/env sh
set -eu

repo="utachicodes/frwrd"
bin_dir="${BIN_DIR:-$HOME/.local/bin}"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "frwrd install: missing required command: $1" >&2
    exit 1
  }
}

need curl
need tar

sha256() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{ print $1 }'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
  else
    echo "frwrd install: missing required command: shasum or sha256sum" >&2
    exit 1
  fi
}

os="$(uname -s)"
arch="$(uname -m)"

case "$os:$arch" in
  Darwin:arm64) target="aarch64-apple-darwin" ;;
  Darwin:x86_64) target="x86_64-apple-darwin" ;;
  Linux:x86_64) target="x86_64-unknown-linux-gnu" ;;
  Linux:aarch64|Linux:arm64) target="aarch64-unknown-linux-gnu" ;;
  *)
    echo "frwrd install: unsupported platform $os/$arch" >&2
    exit 1
    ;;
esac

tmp="$(mktemp -d)"
staged=""
cleanup() {
  rm -rf "$tmp"
  if [ -n "$staged" ]; then
    rm -f "$staged"
  fi
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

api="https://api.github.com/repos/$repo/releases/latest"
asset_url="$(
  curl -fsSL "$api" \
    | sed -n 's/.*"browser_download_url": "\(.*frwrd-v[^"]*-'"$target"'\.tar\.gz\)".*/\1/p' \
    | head -n 1
)"

if [ -z "$asset_url" ]; then
  echo "frwrd install: no release asset found for $target" >&2
  exit 1
fi

echo "Downloading $asset_url"
curl -fsSL "$asset_url" -o "$tmp/frwrd.tar.gz"
curl -fsSL "${asset_url}.sha256" -o "$tmp/frwrd.tar.gz.sha256"

expected="$(awk 'NR == 1 { print $1 }' "$tmp/frwrd.tar.gz.sha256" | tr '[:upper:]' '[:lower:]')"
case "$expected" in
  *[!0-9a-f]*|'')
    echo "frwrd install: release checksum is malformed" >&2
    exit 1
    ;;
esac
if [ "${#expected}" -ne 64 ]; then
  echo "frwrd install: release checksum is malformed" >&2
  exit 1
fi

actual="$(sha256 "$tmp/frwrd.tar.gz")"
if [ "$actual" != "$expected" ]; then
  echo "frwrd install: release checksum verification failed" >&2
  exit 1
fi

echo "Verified SHA-256 checksum"
tar -xzf "$tmp/frwrd.tar.gz" -C "$tmp"

mkdir -p "$bin_dir"
source="$(find "$tmp" -type f -name frwrd -perm -111 | head -n 1)"
if [ -z "$source" ]; then
  echo "frwrd install: release archive does not contain an executable" >&2
  exit 1
fi

staged="$(mktemp "$bin_dir/.frwrd.install.XXXXXX")"
cp "$source" "$staged"
chmod 755 "$staged"

if [ "$os" = "Darwin" ] && command -v xattr >/dev/null 2>&1; then
  if xattr -p com.apple.provenance "$staged" >/dev/null 2>&1; then
    xattr -d com.apple.provenance "$staged"
  fi
fi

mv -f "$staged" "$bin_dir/frwrd"
staged=""

echo "Installed frwrd to $bin_dir/frwrd"
case ":$PATH:" in
  *":$bin_dir:"*) ;;
  *) echo "Add $bin_dir to PATH to run frwrd from any shell." ;;
esac
