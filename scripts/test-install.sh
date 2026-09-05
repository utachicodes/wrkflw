#!/bin/sh

set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

fixture_dir="$tmp_dir/fixtures"
bin_dir="$tmp_dir/bin"
install_dir="$tmp_dir/install"
mkdir -p "$fixture_dir" "$bin_dir"

printf '#!/bin/sh\nprintf "fixture wrkflw\\n"\n' > "$tmp_dir/wrkflw"
chmod +x "$tmp_dir/wrkflw"
tar -C "$tmp_dir" -czf "$fixture_dir/wrkflw_linux_amd64.tar.gz" wrkflw
(
  cd "$fixture_dir"
  sha256sum wrkflw_linux_amd64.tar.gz > checksums.txt
)

cat > "$bin_dir/uname" <<'EOF'
#!/bin/sh
case "$1" in
  -s) printf 'Linux\n' ;;
  -m) printf 'x86_64\n' ;;
  *) exit 1 ;;
esac
EOF

cat > "$bin_dir/curl" <<EOF
#!/bin/sh
url=\$2
output=\$4
cp "$fixture_dir/\${url##*/}" "\$output"
EOF
chmod +x "$bin_dir/uname" "$bin_dir/curl"

PATH="$bin_dir:$PATH" \
WRKFLW_INSTALL_DIR="$install_dir" \
WRKFLW_VERSION=v1.0.0 \
sh "$repo_root/install.sh"

output=$("$install_dir/wrkflw")
[ "$output" = "fixture wrkflw" ] || {
  printf 'installed binary returned %s\n' "$output" >&2
  exit 1
}

printf '#!/bin/sh\nprintf "upgraded wrkflw\\n"\n' > "$tmp_dir/wrkflw"
chmod +x "$tmp_dir/wrkflw"
tar -C "$tmp_dir" -czf "$fixture_dir/wrkflw_linux_amd64.tar.gz" wrkflw
(
  cd "$fixture_dir"
  sha256sum wrkflw_linux_amd64.tar.gz > checksums.txt
)

PATH="$bin_dir:$PATH" \
WRKFLW_INSTALL_DIR="$install_dir" \
WRKFLW_VERSION=v1.0.1 \
sh "$repo_root/install.sh"

output=$("$install_dir/wrkflw")
[ "$output" = "upgraded wrkflw" ] || {
  printf 'upgraded binary returned %s\n' "$output" >&2
  exit 1
}

printf '%064d  wrkflw_linux_amd64.tar.gz\n' 0 > "$fixture_dir/checksums.txt"
if PATH="$bin_dir:$PATH" \
  WRKFLW_INSTALL_DIR="$install_dir" \
  WRKFLW_VERSION=v1.0.2 \
  sh "$repo_root/install.sh"; then
  printf 'installer accepted an invalid checksum\n' >&2
  exit 1
fi

output=$("$install_dir/wrkflw")
[ "$output" = "upgraded wrkflw" ] || {
  printf 'failed upgrade replaced the existing binary\n' >&2
  exit 1
}
