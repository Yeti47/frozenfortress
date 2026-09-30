#!/usr/bin/env bash
# Run the FrozenFortress CLI against the dev stack's database.
# Usage: dev/ffcli.sh user activate <username>
set -euo pipefail

DEV_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$DEV_DIR/.." && pwd)"

export FF_DATABASE_PATH="$DEV_DIR/.data/frozenfortress.db"
export FF_KEY_DIR="$DEV_DIR/.data/keys"
export FF_BACKUP_DIRECTORY="$DEV_DIR/.data/backups"

cd "$REPO_ROOT"
exec go run -tags notesseract ./cli "$@"
