#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT

fixture_dir="$test_root/fixture"
fake_bin="$test_root/bin"
mkdir -p "$fixture_dir/package" "$fake_bin"

printf '#!/bin/sh\nexit 0\n' > "$fixture_dir/package/frwrd"
chmod +x "$fixture_dir/package/frwrd"
tar -C "$fixture_dir" -czf "$fixture_dir/frwrd.tar.gz" package

if command -v shasum >/dev/null 2>&1; then
  digest="$(shasum -a 256 "$fixture_dir/frwrd.tar.gz" | awk '{ print $1 }')"
else
  digest="$(sha256sum "$fixture_dir/frwrd.tar.gz" | awk '{ print $1 }')"
fi
printf '%s  dist/frwrd-v0.0.0-aarch64-apple-darwin.tar.gz\n' "$digest" \
  > "$fixture_dir/frwrd.tar.gz.sha256"
printf '%064d  dist/frwrd-v0.0.0-aarch64-apple-darwin.tar.gz\n' 0 \
  > "$fixture_dir/bad.sha256"

cat > "$fake_bin/uname" <<'EOF'
#!/bin/sh
case "$1" in
  -s) printf '%s\n' "$FAKE_OS" ;;
  -m) printf '%s\n' "$FAKE_ARCH" ;;
  *) exit 1 ;;
esac
EOF

cat > "$fake_bin/curl" <<'EOF'
#!/bin/sh
set -eu

output=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      output="$2"
      shift 2
      ;;
    -*) shift ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

case "$url" in
  */releases/latest)
    printf '{"browser_download_url": "https://example.test/frwrd-v0.0.0-%s.tar.gz"}\n' \
      "$FAKE_TARGET"
    ;;
  *.tar.gz.sha256) cp "$FIXTURE_CHECKSUM" "$output" ;;
  *.tar.gz) cp "$FIXTURE_ARCHIVE" "$output" ;;
  *) exit 1 ;;
esac
EOF

cat > "$fake_bin/xattr" <<'EOF'
#!/bin/sh
if [ "${XATTR_SIGNAL_PARENT:-}" = "1" ] && [ "$1" = "-p" ]; then
  kill -TERM "$PPID"
  exit 0
fi
printf '%s\n' "$*" >> "$XATTR_LOG"
EOF

chmod +x "$fake_bin/uname" "$fake_bin/curl" "$fake_bin/xattr"

run_installer() {
  local os="$1"
  local arch="$2"
  local target="$3"
  local checksum="$4"
  local bin_dir="$5"
  local xattr_log="$6"

  FAKE_OS="$os" \
  FAKE_ARCH="$arch" \
  FAKE_TARGET="$target" \
  FIXTURE_ARCHIVE="$fixture_dir/frwrd.tar.gz" \
  FIXTURE_CHECKSUM="$checksum" \
  XATTR_LOG="$xattr_log" \
  XATTR_SIGNAL_PARENT="${XATTR_SIGNAL_PARENT:-}" \
  BIN_DIR="$bin_dir" \
  PATH="$fake_bin:$PATH" \
    sh "$repo_root/install.sh"
}

macos_bin="$test_root/macos-bin"
macos_xattr="$test_root/macos-xattr.log"
mkdir -p "$macos_bin"
printf 'blocked old binary\n' > "$macos_bin/frwrd"
chmod +x "$macos_bin/frwrd"
old_inode="$(ls -di "$macos_bin/frwrd" | awk '{ print $1 }')"
run_installer \
  Darwin arm64 aarch64-apple-darwin \
  "$fixture_dir/frwrd.tar.gz.sha256" "$macos_bin" "$macos_xattr"
test -x "$macos_bin/frwrd"
new_inode="$(ls -di "$macos_bin/frwrd" | awk '{ print $1 }')"
test "$new_inode" != "$old_inode"
grep -F -- "-d com.apple.provenance $macos_bin/.frwrd.install." "$macos_xattr"
test -z "$(find "$macos_bin" -name '.frwrd.install.*' -print -quit)"

bad_bin="$test_root/bad-bin"
bad_xattr="$test_root/bad-xattr.log"
mkdir -p "$bad_bin"
printf 'working old binary\n' > "$bad_bin/frwrd"
chmod +x "$bad_bin/frwrd"
if run_installer \
  Darwin arm64 aarch64-apple-darwin \
  "$fixture_dir/bad.sha256" "$bad_bin" "$bad_xattr"; then
  echo "installer accepted a mismatched checksum" >&2
  exit 1
fi
grep -F 'working old binary' "$bad_bin/frwrd"
test ! -e "$bad_xattr"

interrupt_bin="$test_root/interrupt-bin"
interrupt_xattr="$test_root/interrupt-xattr.log"
mkdir -p "$interrupt_bin"
printf 'working old binary\n' > "$interrupt_bin/frwrd"
chmod +x "$interrupt_bin/frwrd"
set +e
XATTR_SIGNAL_PARENT=1 run_installer \
  Darwin arm64 aarch64-apple-darwin \
  "$fixture_dir/frwrd.tar.gz.sha256" "$interrupt_bin" "$interrupt_xattr"
interrupt_status=$?
set -e
test "$interrupt_status" -eq 143
grep -F 'working old binary' "$interrupt_bin/frwrd"
test -z "$(find "$interrupt_bin" -name '.frwrd.install.*' -print -quit)"

linux_bin="$test_root/linux-bin"
linux_xattr="$test_root/linux-xattr.log"
run_installer \
  Linux x86_64 x86_64-unknown-linux-gnu \
  "$fixture_dir/frwrd.tar.gz.sha256" "$linux_bin" "$linux_xattr"
test -x "$linux_bin/frwrd"
test ! -e "$linux_xattr"
