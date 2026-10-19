#!/bin/sh

set -eu

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname "$0")" && pwd)
SERVER_DIR=$(CDPATH='' cd -- "$SCRIPT_DIR/.." && pwd)

SMOKE_PORT="${SMOKE_PORT:-18080}"
SMOKE_BASE_URL="${SMOKE_BASE_URL:-http://127.0.0.1:${SMOKE_PORT}}"
SMOKE_TIMEOUT_SECONDS="${SMOKE_TIMEOUT_SECONDS:-30}"
BOOTSTRAP_DB="${INK_SMOKE_BOOTSTRAP_DB:-1}"
SMOKE_RUN_ID="$(date +%s)-$$"
SMOKE_PRINTER_ID="printer_smoke_${SMOKE_RUN_ID}"
SMOKE_DB_CONTAINER=""
SMOKE_DIR=""
export INK_SMOKE_PRINTER_ID="$SMOKE_PRINTER_ID"

cleanup() {
  if [ "${SERVER_PID:-}" != "" ] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  if [ "${FIXTURE_SETUP:-0}" = "1" ]; then
    (cd "$SERVER_DIR" && go run ./cmd/smoke-fixture cleanup) >/dev/null 2>&1 || true
  fi
  if [ -n "$SMOKE_DB_CONTAINER" ]; then
    docker stop "$SMOKE_DB_CONTAINER" >/dev/null 2>&1 || true
  fi
  if [ -n "$SMOKE_DIR" ]; then
    rm -rf "$SMOKE_DIR"
  fi
}

trap cleanup EXIT INT TERM

python3 - "$SMOKE_PORT" "$SMOKE_BASE_URL" <<'PY'
import socket
import sys
import urllib.parse

try:
    port = int(sys.argv[1])
    url = urllib.parse.urlsplit(sys.argv[2])
    if not 1 <= port <= 65535:
        raise ValueError("invalid port")
    if (
        url.scheme != "http"
        or url.hostname not in ("127.0.0.1", "localhost")
        or url.port != port
        or url.path or url.query or url.fragment or url.username or url.password
    ):
        raise ValueError("base URL must point to the smoke API")
except ValueError:
    raise SystemExit("SMOKE_BASE_URL must be http://127.0.0.1:SMOKE_PORT (or localhost) with a valid port.")
try:
    with socket.socket() as listener:
        listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        listener.bind(("127.0.0.1", port))
except OSError:
    raise SystemExit("Smoke API port is already in use; choose a free SMOKE_PORT.")
PY

case "$BOOTSTRAP_DB" in
  0)
    if [ -z "${DATABASE_URL:-}" ]; then
      echo "INK_SMOKE_BOOTSTRAP_DB=0 requires DATABASE_URL for an empty, disposable test database." >&2
      exit 1
    fi
    ;;
  1)
    if ! docker info >/dev/null 2>&1; then
      echo "Docker is required for the isolated smoke database. Start Docker Desktop or OrbStack first." >&2
      exit 1
    fi
    SMOKE_DB_CONTAINER="ink-smoke-postgres-${SMOKE_RUN_ID}"
    docker run --rm --detach \
      --name "$SMOKE_DB_CONTAINER" \
      --env POSTGRES_DB=ink \
      --env POSTGRES_USER=postgres \
      --env POSTGRES_PASSWORD=postgres \
      --publish 127.0.0.1::5432 \
      postgres:16 >/dev/null

    attempt=0
    until docker exec "$SMOKE_DB_CONTAINER" pg_isready -h 127.0.0.1 -U postgres -d ink >/dev/null 2>&1; do
      attempt=$((attempt + 1))
      if [ "$attempt" -ge 30 ]; then
        echo "Temporary PostgreSQL did not become ready." >&2
        docker logs "$SMOKE_DB_CONTAINER" >&2 || true
        exit 1
      fi
      sleep 1
    done

    smoke_db_address=$(docker port "$SMOKE_DB_CONTAINER" 5432/tcp)
    smoke_db_port=${smoke_db_address##*:}
    case "$smoke_db_port" in
      '' | *[!0-9]*)
        echo "Could not determine the temporary PostgreSQL port." >&2
        exit 1
        ;;
    esac
    DATABASE_URL="postgres://postgres:postgres@127.0.0.1:${smoke_db_port}/ink?sslmode=disable"
    ;;
  *)
    echo "INK_SMOKE_BOOTSTRAP_DB must be 0 or 1." >&2
    exit 1
    ;;
esac

umask 077
SMOKE_DIR=$(mktemp -d "${TMPDIR:-/tmp}/ink-smoke.XXXXXX")
CREDENTIALS_FILE="$SMOKE_DIR/admin-credentials"
export DATABASE_URL
export INK_DEV_ADMIN_CREDENTIALS_PATH="$CREDENTIALS_FILE"
JWT_SECRET=$(openssl rand -hex 32)
AI_CONFIG_ENCRYPTION_KEY=$(openssl rand -base64 32 | tr -d '\n')
export JWT_SECRET AI_CONFIG_ENCRYPTION_KEY
export MEMOBIRD_ACCESS_KEY=""
export PRINT_STATUS_SYNC_ENABLED=false

(
  cd "$SERVER_DIR"
  go run ./cmd/migrate up
  INK_TEST_DATABASE_URL="$DATABASE_URL" \
    go test -race ./internal/platform/store/postgres ./internal/platform/migrate -count=1
  go run ./cmd/seed dev
  go run ./cmd/smoke-fixture cleanup
  go run ./cmd/smoke-fixture setup
)
FIXTURE_SETUP=1

if [ ! -r "$CREDENTIALS_FILE" ]; then
  echo "Smoke credentials were not created; use an empty, disposable test database." >&2
  exit 1
fi

LOGIN_NAME=$(awk -F= '$1=="login" {print $2}' "$CREDENTIALS_FILE")
LOGIN_PASSWORD=$(awk -F= '$1=="password" {print $2}' "$CREDENTIALS_FILE")

if [ -z "$LOGIN_NAME" ] || [ -z "$LOGIN_PASSWORD" ]; then
  echo "Invalid credentials file: $CREDENTIALS_FILE" >&2
  exit 1
fi

SERVER_LOG="$SMOKE_DIR/server.log"
SERVER_BINARY="$SMOKE_DIR/api"
WORKSPACE_BEFORE="$SMOKE_DIR/workspace-before.json"
WORKSPACE_AFTER="$SMOKE_DIR/workspace-after.json"
WORKSPACE_RESTORED="$SMOKE_DIR/workspace-restored.json"
LOGIN_JSON="$SMOKE_DIR/login.json"
UPDATED_PAYLOAD="$SMOKE_DIR/workspace-updated.json"
CONFIRMATION_PAYLOAD="$SMOKE_DIR/workspace-confirmation.json"
PRINT_STATUS_JSON="$SMOKE_DIR/print-status.json"
PRINT_DETAIL_JSON="$SMOKE_DIR/print-detail.json"

(cd "$SERVER_DIR" && go build -o "$SERVER_BINARY" ./cmd/api)

(
  cd "$SERVER_DIR"
  exec env \
    PORT="$SMOKE_PORT" \
    WORKERS_ENABLED=false \
    "$SERVER_BINARY" >"$SERVER_LOG" 2>&1
) &
SERVER_PID=$!

attempt=0
while :; do
  if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    echo "Smoke API exited before becoming healthy. Recent log output:" >&2
    tail -n 40 "$SERVER_LOG" >&2 || true
    exit 1
  fi
  if curl --silent --show-error --fail --max-time 2 "$SMOKE_BASE_URL/healthz" >/dev/null 2>&1; then
    if kill -0 "$SERVER_PID" >/dev/null 2>&1; then
      break
    fi
    continue
  fi
  attempt=$((attempt + 1))
  if [ "$attempt" -ge "$SMOKE_TIMEOUT_SECONDS" ]; then
    echo "API failed to start in time. Recent log output:" >&2
    tail -n 40 "$SERVER_LOG" >&2 || true
    exit 1
  fi
  sleep 1
done

curl --silent --show-error --fail \
  -H "Content-Type: application/json" \
  -X POST \
  -d "{\"email\":\"$LOGIN_NAME\",\"password\":\"$LOGIN_PASSWORD\"}" \
  "$SMOKE_BASE_URL/api/v1/auth/login" >"$LOGIN_JSON"

ACCESS_TOKEN=$(
  python3 - "$LOGIN_JSON" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)

token = payload.get("accessToken", "")
if not token:
    raise SystemExit("missing access token")

print(token)
PY
)

AUTH_HEADER="Authorization: Bearer $ACCESS_TOKEN"

curl --silent --show-error --fail \
  -H "$AUTH_HEADER" \
  "$SMOKE_BASE_URL/api/v1/workspace" >"$WORKSPACE_BEFORE"

python3 - "$WORKSPACE_BEFORE" "$UPDATED_PAYLOAD" <<'PY'
import json
import sys

source_path, target_path = sys.argv[1], sys.argv[2]
with open(source_path, "r", encoding="utf-8") as fh:
    workspace = json.load(fh)

preferences = workspace.setdefault("preferences", {})
preferences["sendConfirmationEnabled"] = not bool(preferences.get("sendConfirmationEnabled", False))

with open(target_path, "w", encoding="utf-8") as fh:
    json.dump(workspace, fh, ensure_ascii=False)
PY

curl --silent --show-error --fail \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -X PUT \
  --data-binary "@$UPDATED_PAYLOAD" \
  "$SMOKE_BASE_URL/api/v1/workspace" >"$WORKSPACE_AFTER"

python3 - "$WORKSPACE_BEFORE" "$WORKSPACE_AFTER" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    before = json.load(fh)
with open(sys.argv[2], "r", encoding="utf-8") as fh:
    after = json.load(fh)

before_flag = bool(before.get("preferences", {}).get("sendConfirmationEnabled", False))
after_flag = bool(after.get("preferences", {}).get("sendConfirmationEnabled", False))

if before_flag == after_flag:
    raise SystemExit("workspace update did not persist the expected change")
if after.get("revision") != before.get("revision", 0) + 1:
    raise SystemExit("workspace revision did not advance")
PY

stale_status=$(curl --silent --show-error --output "$SMOKE_DIR/workspace-conflict.json" --write-out '%{http_code}' \
  -H "$AUTH_HEADER" -H "Content-Type: application/json" \
  -X PUT --data-binary "@$UPDATED_PAYLOAD" "$SMOKE_BASE_URL/api/v1/workspace")
if [ "$stale_status" != "409" ]; then
  echo "Stale workspace save was not rejected." >&2
  exit 1
fi

python3 - "$WORKSPACE_AFTER" "$SMOKE_DIR/workspace-unversioned.json" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as fh:
    workspace = json.load(fh)
workspace.pop("revision", None)
with open(sys.argv[2], "w", encoding="utf-8") as fh:
    json.dump(workspace, fh)
PY
unversioned_status=$(curl --silent --show-error --output "$SMOKE_DIR/workspace-revision-error.json" --write-out '%{http_code}' \
  -H "$AUTH_HEADER" -H "Content-Type: application/json" \
  -X PUT --data-binary "@$SMOKE_DIR/workspace-unversioned.json" "$SMOKE_BASE_URL/api/v1/workspace")
if [ "$unversioned_status" != "428" ]; then
  echo "Unversioned workspace save was not rejected." >&2
  exit 1
fi
curl --silent --show-error --fail -H "$AUTH_HEADER" \
  "$SMOKE_BASE_URL/api/v1/workspace" >"$SMOKE_DIR/workspace-after-rejections.json"
python3 - "$WORKSPACE_AFTER" "$SMOKE_DIR/workspace-after-rejections.json" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as fh:
    expected = json.load(fh)
with open(sys.argv[2], encoding="utf-8") as fh:
    actual = json.load(fh)
if expected != actual:
    raise SystemExit("Rejected saves changed the workspace or its revision")
PY

python3 - "$WORKSPACE_AFTER" "$CONFIRMATION_PAYLOAD" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    workspace = json.load(fh)
workspace.setdefault("preferences", {})["sendConfirmationEnabled"] = True
with open(sys.argv[2], "w", encoding="utf-8") as fh:
    json.dump(workspace, fh, ensure_ascii=False)
PY

curl --silent --show-error --fail \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -X PUT \
  --data-binary "@$CONFIRMATION_PAYLOAD" \
  "$SMOKE_BASE_URL/api/v1/workspace" >"$WORKSPACE_AFTER"

curl --silent --show-error --fail \
  -H "$AUTH_HEADER" -H "Content-Type: application/json" \
  -X POST -d '{"title":"Smoke preview","content":"Hello from Ink"}' \
  "$SMOKE_BASE_URL/api/v1/print-preview" >"$SMOKE_DIR/preview.json"
python3 - "$SMOKE_DIR/preview.json" <<'PY'
import base64
import json
import struct
import sys
with open(sys.argv[1], encoding="utf-8") as fh:
    image = base64.b64decode(json.load(fh)["image"], validate=True)
if image[:8] != b"\x89PNG\r\n\x1a\n":
    raise SystemExit("Preview did not return a PNG")
width, height = struct.unpack(">II", image[16:24])
if width != 384 or height > 8192:
    raise SystemExit("Preview dimensions exceed the printing limit")
PY

for removed_path in /api/v1/plugins /api/v1/admin/plugins /api/v1/print-schedules; do
  removed_status=$(curl --silent --show-error -o /dev/null -w '%{http_code}' -H "$AUTH_HEADER" "$SMOKE_BASE_URL$removed_path")
  test "$removed_status" = 404
done
python3 - "$SMOKE_DIR/first-print.json" "$SMOKE_PRINTER_ID" <<'PY_JOB'
import json
import sys
with open(sys.argv[1], "w", encoding="utf-8") as fh:
    json.dump({"title":"First manual print","source":"Manual","content":"Synthetic preserved body","printerBindingId":sys.argv[2],"submitImmediately":False},fh)
PY_JOB
curl --silent --show-error --fail -H "$AUTH_HEADER" -H "Content-Type: application/json" -X POST --data-binary "@$SMOKE_DIR/first-print.json" "$SMOKE_BASE_URL/api/v1/print-jobs" >"$SMOKE_DIR/first-created.json"
PRINT_JOB_ID=$(python3 - "$SMOKE_DIR/first-created.json" <<'PY_JOB_ID'
import json
import sys
with open(sys.argv[1],encoding="utf-8") as fh:
    job=json.load(fh)["printJob"]
if job["status"] != "pending":
    raise SystemExit("Manual fixture unexpectedly contacted provider")
print(job["id"])
PY_JOB_ID
)

python3 - "$SMOKE_DIR/manual-print.json" "$SMOKE_PRINTER_ID" <<'PY'
import json
import sys
with open(sys.argv[1], "w", encoding="utf-8") as fh:
    json.dump({"title": "Pagination smoke fixture", "source": "Manual", "content": "Synthetic content", "printerBindingId": sys.argv[2], "submitImmediately": False}, fh)
PY
curl --silent --show-error --fail \
  -H "$AUTH_HEADER" -H "Content-Type: application/json" \
  -X POST --data-binary "@$SMOKE_DIR/manual-print.json" \
  "$SMOKE_BASE_URL/api/v1/print-jobs" >"$SMOKE_DIR/manual-created.json"
curl --silent --show-error --fail -H "$AUTH_HEADER" \
  "$SMOKE_BASE_URL/api/v1/print-jobs?limit=1" >"$SMOKE_DIR/page-first.json"
PRINT_PAGE_CURSOR=$(python3 - "$SMOKE_DIR/page-first.json" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as fh:
    page = json.load(fh)
if len(page.get("printJobs", [])) != 1 or not page.get("nextCursor"):
    raise SystemExit("small print page did not provide a continuation cursor")
print(page["nextCursor"])
PY
)
curl --silent --show-error --fail -H "$AUTH_HEADER" --get \
  --data-urlencode "limit=1" --data-urlencode "cursor=$PRINT_PAGE_CURSOR" \
  "$SMOKE_BASE_URL/api/v1/print-jobs" >"$SMOKE_DIR/page-second.json"
python3 - "$SMOKE_DIR/page-first.json" "$SMOKE_DIR/page-second.json" "$SMOKE_DIR/manual-created.json" "$PRINT_JOB_ID" <<'PY'
import json
import sys
pages = []
for path in sys.argv[1:3]:
    with open(path, encoding="utf-8") as fh:
        pages.append(json.load(fh))
with open(sys.argv[3], encoding="utf-8") as fh:
    manual = json.load(fh)["printJob"]
rows = [job for page in pages for job in page.get("printJobs", [])]
if len(rows) != 2 or any("content" in job for job in rows) or {job["id"] for job in rows} != {manual["id"], sys.argv[4]} or pages[1].get("nextCursor") is not None:
    raise SystemExit("cursor continuation lost or duplicated print summaries")
PY

curl --silent --show-error --fail \
  -H "$AUTH_HEADER" \
  "$SMOKE_BASE_URL/api/v1/print-jobs/status?ids=$PRINT_JOB_ID" >"$PRINT_STATUS_JSON"

curl --silent --show-error --fail \
  -H "$AUTH_HEADER" \
  "$SMOKE_BASE_URL/api/v1/print-jobs/$PRINT_JOB_ID" >"$PRINT_DETAIL_JSON"

python3 - "$PRINT_STATUS_JSON" "$PRINT_DETAIL_JSON" "$PRINT_JOB_ID" <<'PY'
import json
import sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    status = json.load(fh)
with open(sys.argv[2], "r", encoding="utf-8") as fh:
    detail = json.load(fh).get("printJob", {})
rows = status.get("printJobs", [])
if len(rows) != 1 or rows[0].get("id") != sys.argv[3] or rows[0].get("status") != "pending" or "content" in rows[0]:
    raise SystemExit("lightweight status response is missing or contains print content")
if status.get("counts", {}).get("pending", 0) < 1 or not status.get("latestJobId"):
    raise SystemExit("global print status metadata is missing")
if detail.get("id") != sys.argv[3] or not detail.get("content"):
    raise SystemExit("print detail did not preserve the job body")
PY

python3 - "$WORKSPACE_BEFORE" "$WORKSPACE_AFTER" "$SMOKE_DIR/workspace-restore-payload.json" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as fh:
    original = json.load(fh)
with open(sys.argv[2], encoding="utf-8") as fh:
    current = json.load(fh)
original["revision"] = current["revision"]
with open(sys.argv[3], "w", encoding="utf-8") as fh:
    json.dump(original, fh)
PY
curl --silent --show-error --fail \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -X PUT \
  --data-binary "@$SMOKE_DIR/workspace-restore-payload.json" \
  "$SMOKE_BASE_URL/api/v1/workspace" >"$WORKSPACE_RESTORED"

python3 - "$WORKSPACE_BEFORE" "$WORKSPACE_RESTORED" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    expected = json.load(fh)
with open(sys.argv[2], "r", encoding="utf-8") as fh:
    restored = json.load(fh)

expected.pop("revision", None)
restored.pop("revision", None)
if expected != restored:
    raise SystemExit("workspace state was not restored after smoke test")
PY

echo "Smoke test passed: auth, workspace persistence and conflict protection, PNG preview, removed extension endpoints, manual printing, and paginated print/status/detail reads succeeded."
