#!/usr/bin/env bash
# Activate the repository's git hooks (secret scanning with gitleaks).
# Usage: ./setup-hooks.sh [--install]
#   --install  also download the pinned, checksum-verified gitleaks into ~/.local/bin
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=.githooks/gitleaks.env
. .githooks/gitleaks.env

if [ "${1:-}" = "--install" ]; then
  .githooks/install-gitleaks.sh
fi

git config core.hooksPath .githooks
echo "Git hooks enabled (core.hooksPath=.githooks)."

PATH="$HOME/.local/bin:$PATH"
if ! command -v gitleaks >/dev/null 2>&1; then
  echo "gitleaks is not installed yet: commits will be blocked until it is. Run ./setup-hooks.sh --install" >&2
  exit 1
fi
echo "Found $(gitleaks version) (repo pins $GITLEAKS_VERSION)."
