#!/bin/sh

# Fast pre-push verification.
#
# Runs the same checks as CI, but natively and scoped to the paths that changed
# since the merge base with origin/main. The full workflow emulation
# (`make check-local-ci`) stays available as a manual command for changes that
# touch the workflows themselves or before a release.

set -eu

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname "$0")" && pwd)
REPO_DIR=$(CDPATH='' cd -- "$SCRIPT_DIR/.." && pwd)

cd "$REPO_DIR"

BASE_REF="${INK_PUSH_BASE_REF:-origin/main}"

merge_base() {
  if git rev-parse --verify --quiet "$BASE_REF" >/dev/null 2>&1; then
    git merge-base "$BASE_REF" HEAD 2>/dev/null && return
  fi
  # No remote branch yet (first push): diff against the root commit instead.
  git rev-list --max-parents=0 HEAD 2>/dev/null | tail -n 1
}

BASE=$(merge_base || true)
if [ -z "$BASE" ]; then
  echo "==> Cannot determine a base commit; running every check"
  CHANGED=$(git ls-files)
else
  CHANGED=$(git diff --name-only "$BASE" HEAD)
fi

if [ -z "$CHANGED" ]; then
  echo "==> No file changes to check"
  exit 0
fi

changed_matches() {
  printf '%s\n' "$CHANGED" | grep -Eq "$1"
}

run_web=0
run_server=0
run_lint=0
run_smoke=0
run_ci=0

if changed_matches '^(web/|web$|Makefile$)'; then
  run_web=1
fi
if changed_matches '^(server/|scripts/|Makefile$)'; then
  run_server=1
  run_lint=1
fi
if changed_matches '^\.github/workflows/'; then
  run_ci=1
fi
# Smoke coverage only matters for the surfaces it exercises: auth, workspace
# persistence, bootstrap, seeding and printing.
if changed_matches \
  '^(server/internal/(auth|workspace|content|printer|user|session)/|server/internal/platform/(store|migrate|seed|token|password|secret)/|server/migrations/|server/cmd/|server/scripts/|Makefile$)'; then
  run_smoke=1
fi

if [ "$run_ci" = "1" ]; then
  echo "==> Workflow definitions changed; running full local CI via act"
  make check-local-ci
  exit 0
fi

if [ "$run_web" = "0" ] && [ "$run_server" = "0" ]; then
  echo "==> No web or server changes; skipping native checks"
  exit 0
fi

if [ "$run_web" = "1" ]; then
  echo "==> Running web checks"
  make check-web
fi

if [ "$run_server" = "1" ]; then
  echo "==> Running server checks"
  make check-api
fi

if [ "$run_lint" = "1" ]; then
  echo "==> Running server lint"
  make lint-api
fi

if [ "$run_smoke" = "1" ] && [ "${INK_SKIP_SMOKE:-0}" != "1" ]; then
  echo "==> Running API smoke test"
  make smoke-api
fi
