#!/usr/bin/env bash
# Start the local dev stack (Aspire). See doc/dev-stack.md.
# Extra arguments are passed to `aspire run`, e.g. ./dev.sh --detach
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/dev"

if [[ ! -d node_modules ]]; then
    npm ci
fi

exec aspire run "$@"
