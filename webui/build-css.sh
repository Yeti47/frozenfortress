#!/usr/bin/env bash
# Build webui/static/css/app.css from tailwind-src.css using the pinned
# Tailwind v4 standalone CLI. The binary is downloaded once into
# webui/.tools/tailwindcss and reused. See webui/static/VENDORED.md for the
# pinned version and SHA-256.
set -euo pipefail

# Resolve repo root from this script's location
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TOOLS_DIR="$REPO_ROOT/webui/.tools"
BIN="$TOOLS_DIR/tailwindcss"

# Pinned per webui/static/VENDORED.md — keep in sync.
TAILWIND_VER="v4.1.14"
TAILWIND_SHA256="bc34c301b080b6e6b98ed24118419833f966f6f347e556945d6557d36a44a56e"

INPUT="$REPO_ROOT/webui/static/css/tailwind-src.css"
OUTPUT="$REPO_ROOT/webui/static/css/app.css"

uname_m="$(uname -m)"
uname_s="$(uname -s)"
case "$uname_s-$uname_m" in
  Linux-x86_64)  ASSET="tailwindcss-linux-x64" ;;
  Linux-aarch64) ASSET="tailwindcss-linux-arm64" ;;
  Darwin-x86_64) ASSET="tailwindcss-macos-x64"; TAILWIND_SHA256="" ;;
  Darwin-arm64)  ASSET="tailwindcss-macos-arm64"; TAILWIND_SHA256="" ;;
  *) echo "Unsupported platform: $uname_s-$uname_m" >&2; exit 1 ;;
esac

mkdir -p "$TOOLS_DIR"

download_binary() {
  echo "Downloading Tailwind CSS $TAILWIND_VER ($ASSET) ..."
  curl -fsSL -o "$BIN" \
    "https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWIND_VER}/${ASSET}"
  chmod +x "$BIN"
}

verify_binary() {
  if [[ -z "$TAILWIND_SHA256" ]]; then
    echo "Note: no pinned SHA-256 for $ASSET; skipping verification." >&2
    return 0
  fi
  local actual
  actual="$(sha256sum "$BIN" | awk '{print $1}')"
  if [[ "$actual" != "$TAILWIND_SHA256" ]]; then
    echo "Tailwind binary SHA-256 mismatch!" >&2
    echo "  expected: $TAILWIND_SHA256" >&2
    echo "  actual:   $actual" >&2
    return 1
  fi
}

if [[ ! -x "$BIN" ]]; then
  download_binary
fi
if ! verify_binary; then
  echo "Re-downloading to recover from corrupted/stale binary ..." >&2
  rm -f "$BIN"
  download_binary
  verify_binary
fi

echo "Building $OUTPUT ..."
"$BIN" -i "$INPUT" -o "$OUTPUT" --minify
echo "Built $(wc -c <"$OUTPUT") bytes."
