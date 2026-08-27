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
SMOKE_DB_CONTAINER=""

cleanup() {
  if [ -n "$SMOKE_DB_CONTAINER" ]; then
    docker stop "$SMOKE_DB_CONTAINER" >/dev/null 2>&1 || true
  fi
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
ensure_action_cache actions/setup-node v7
ensure_action_cache actions/setup-go v7
ensure_action_cache golangci/golangci-lint-action v9

run_act() {
  act \
    --action-offline-mode \
    --container-architecture linux/amd64 \
    -P ubuntu-latest=catthehacker/ubuntu:act-latest \
    "$@"
}

run_api_smoke() {
  SMOKE_DB_CONTAINER="ink-local-ci-postgres-$(date +%s)-$$"

  docker run --rm --detach \
    --name "$SMOKE_DB_CONTAINER" \
    --env POSTGRES_DB=ink \
    --env POSTGRES_USER=postgres \
    --env POSTGRES_PASSWORD=postgres \
    --publish 127.0.0.1::5432 \
    postgres:16 >/dev/null

  attempt=0
  until docker exec "$SMOKE_DB_CONTAINER" pg_isready -U postgres -d ink >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 30 ]; then
      echo "Temporary PostgreSQL did not become ready." >&2
      docker logs "$SMOKE_DB_CONTAINER" >&2 || true
      return 1
    fi
    sleep 1
  done

  smoke_db_address=$(docker port "$SMOKE_DB_CONTAINER" 5432/tcp)
  smoke_db_port=${smoke_db_address##*:}
  case "$smoke_db_port" in
    '' | *[!0-9]*)
      echo "Could not determine the temporary PostgreSQL port." >&2
      return 1
      ;;
  esac

  DATABASE_URL="postgres://postgres:postgres@127.0.0.1:${smoke_db_port}/ink?sslmode=disable" \
    INK_SMOKE_BOOTSTRAP_DB=0 \
    make smoke-api
}

echo "==> Running web-quality via act"
run_act pull_request -W .github/workflows/web-quality.yml

echo "==> Running server-check via act"
run_act pull_request -W .github/workflows/server-quality.yml -j server-check

echo "==> Running golangci-lint via act"
run_act pull_request -W .github/workflows/server-quality.yml -j golangci-lint

echo "==> Running api-smoke locally"
echo "act 0.2.87 currently panics on this repo's service-container smoke job, so we run the local equivalent instead."
echo "The smoke test uses an isolated temporary PostgreSQL container and leaves the development database untouched."
run_api_smoke
