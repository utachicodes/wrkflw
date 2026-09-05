#!/usr/bin/env sh
# One-shot gateway setup. Everything configurable lives in the wrkflw app
# (Settings -> Messaging); this script only prepares the machine:
# installs frwrd, creates the assistant repository, and writes the minimal
# local file that points at the control plane.
#
# Usage:
#   WRKFLW_BASE_URL="https://wrkflw" WRKFLW_API_TOKEN="wrkflw_..." \
#     sh scripts/setup-gateway.sh [--assistant-dir ~/Code/assistant]
#
# WRKFLW_API_TOKEN is required. Create it in the app under Settings ->
# Messaging (Create token) or Settings -> API access. For a local server,
# set WRKFLW_BASE_URL to it, e.g. http://127.0.0.1:8080.
set -eu

assistant_dir="$HOME/Code/assistant"
for arg in "$@"; do
  case "$arg" in
    --assistant-dir=*) assistant_dir="${arg#--assistant-dir=}";;
    -h|--help) echo "usage: WRKFLW_API_TOKEN=wrkflw_... sh scripts/setup-gateway.sh [--assistant-dir=DIR]"; exit 0;;
    *) echo "unknown argument: $arg" >&2; exit 1;;
  esac
done

if [ -z "${WRKFLW_API_TOKEN:-}" ]; then
  echo "WRKFLW_API_TOKEN is required; create one in the app under Settings -> Messaging." >&2
  exit 1
fi
base_url="${WRKFLW_BASE_URL:-https://wrkflw}"
frwrd_home="${FRWRD_HOME:-$HOME/.frwrd}"
config="$frwrd_home/config.toml"

if ! command -v frwrd >/dev/null 2>&1; then
  if command -v cargo >/dev/null 2>&1 && [ -f "frwrd/Cargo.toml" ]; then
    echo "Building frwrd from source..."
    (cd frwrd && cargo build --locked --release)
    install_dir="$HOME/.local/bin"
    mkdir -p "$install_dir"
    install -m 755 "frwrd/target/release/frwrd" "$install_dir/frwrd"
    export PATH="$install_dir:$PATH"
  else
    echo "frwrd is not on PATH. Install it first:" >&2
    echo "  cd frwrd && cargo build --locked --release" >&2
    exit 1
  fi
fi

if [ ! -f "$config" ]; then
  echo "Creating assistant repository at $assistant_dir..."
  FRWRD_HOME="$frwrd_home" frwrd init "$assistant_dir"
  cat > "$config" <<EOF
# Managed by the wrkflw app (Settings -> Messaging). This file keeps only
# what the server cannot know: where the assistant lives and who may ask.
assistant_root = "$assistant_dir"

[wrkflw]
base_url = "$base_url"
token = "$WRKFLW_API_TOKEN"
pull_config = true
mirror = true
EOF
  chmod 600 "$config"
  echo "Wrote $config"
else
  echo "$config already exists; leaving it untouched."
  echo "Make sure it contains, under [wrkflw]: base_url, token, pull_config = true, mirror = true."
fi

echo "Validating..."
FRWRD_HOME="$frwrd_home" frwrd doctor
echo ""
echo "Next: configure the channel in the app (Settings -> Messaging), then run:"
echo "  FRWRD_HOME=\"$frwrd_home\" frwrd"
