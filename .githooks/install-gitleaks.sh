#!/usr/bin/env bash
# Download the pinned gitleaks release, verify its SHA-256 and install the binary.
# Usage: .githooks/install-gitleaks.sh [install-dir]   (default: ~/.local/bin)
# Used by setup-hooks.sh and CI so both run exactly the same verified binary.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=gitleaks.env
. "$here/gitleaks.env"

dest="${1:-$HOME/.local/bin}"

case "$(uname -s)-$(uname -m)" in
  Linux-x86_64)               platform=linux_x64;   expected="$GITLEAKS_SHA256_LINUX_X64" ;;
  Linux-aarch64|Linux-arm64)  platform=linux_arm64; expected="$GITLEAKS_SHA256_LINUX_ARM64" ;;
  Darwin-x86_64)              platform=darwin_x64;  expected="$GITLEAKS_SHA256_DARWIN_X64" ;;
  Darwin-arm64)               platform=darwin_arm64; expected="$GITLEAKS_SHA256_DARWIN_ARM64" ;;
  *) echo "Unsupported platform $(uname -s)-$(uname -m). Install gitleaks v$GITLEAKS_VERSION manually: https://github.com/gitleaks/gitleaks/releases/tag/v$GITLEAKS_VERSION" >&2; exit 1 ;;
esac

archive="gitleaks_${GITLEAKS_VERSION}_${platform}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl --fail --silent --show-error --location \
  --output "$tmp/$archive" \
  "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/${archive}"

actual="$(sha256sum "$tmp/$archive" 2>/dev/null | cut -d' ' -f1 || shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)"
if [ "$actual" != "$expected" ]; then
  echo "Checksum mismatch for $archive" >&2
  echo "  expected: $expected" >&2
  echo "  actual:   $actual" >&2
  exit 1
fi

tar -xzf "$tmp/$archive" -C "$tmp" gitleaks
mkdir -p "$dest"
install -m 0755 "$tmp/gitleaks" "$dest/gitleaks"
echo "Installed gitleaks v$GITLEAKS_VERSION to $dest/gitleaks"
