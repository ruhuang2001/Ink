#!/bin/sh
# Real API + disposable PostgreSQL; synthetic memory/AI HTTP peers only.
set -eu
SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname "$0")" && pwd)
exec python3 "$SCRIPT_DIR/memory_e2e.py" "$@"
