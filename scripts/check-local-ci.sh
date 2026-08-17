#!/bin/sh

set -eu

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname "$0")" && pwd)
REPO_DIR=$(CDPATH='' cd -- "$SCRIPT_DIR/.." && pwd)

cd "$REPO_DIR"

if ! command -v act >/dev/null 2>&1; then
  echo "Missing act. Install it first with 'brew install act'." >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "Docker is not running. Start Docker Desktop or OrbStack first." >&2
  exit 1
fi

TMP_GIT_CONFIG=$(mktemp "${TMPDIR:-/tmp}/ink-act-gitconfig.XXXXXX")

cleanup() {
  rm -f "$TMP_GIT_CONFIG"
}

trap cleanup EXIT INT TERM

if [ -f "$HOME/.gitconfig" ]; then
  git config --file "$TMP_GIT_CONFIG" include.path "$HOME/.gitconfig"
fi

git config --file "$TMP_GIT_CONFIG" url.git@github.com:.insteadOf https://github.com/
export GIT_CONFIG_GLOBAL="$TMP_GIT_CONFIG"

ACT_CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/act"
mkdir -p "$ACT_CACHE_DIR"

ensure_action_cache() {
  repo="$1"
  ref="$2"
  cache_dir="$ACT_CACHE_DIR/$(printf '%s@%s' "$(printf '%s' "$repo" | tr '/' '-')" "$ref")"

  if [ -d "$cache_dir/.git" ]; then
    return
  fi

  rm -rf "$cache_dir"
  echo "==> Seeding $repo@$ref into act cache"
  git clone --depth 1 --branch "$ref" "git@github.com:$repo" "$cache_dir" >/dev/null
}

ensure_action_cache actions/checkout v7
ensure_action_cache actions/setup-node v6
ensure_action_cache actions/setup-go v7
ensure_action_cache golangci/golangci-lint-action v9

run_act() {
  act \
    --action-offline-mode \
    --container-architecture linux/amd64 \
    -P ubuntu-latest=catthehacker/ubuntu:act-latest \
    "$@"
}

echo "==> Running web-quality via act"
run_act pull_request -W .github/workflows/web-quality.yml

echo "==> Running server-check via act"
run_act pull_request -W .github/workflows/server-quality.yml -j server-check

echo "==> Running golangci-lint via act"
run_act pull_request -W .github/workflows/server-quality.yml -j golangci-lint

echo "==> Running api-smoke locally"
echo "act 0.2.87 currently panics on this repo's service-container smoke job, so we run the local equivalent instead."
make smoke-api
