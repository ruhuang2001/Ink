# Backup and restore verification

On 2026-10-08, a PostgreSQL 16 custom-format dump was restored into a second, empty test database. All 13 public tables matched, including migrations 001–010, workspace revision 7, three complete print bodies, an offline printer binding, its disabled schedule, and delivery history. No development or production database was used.

This checks database recovery with synthetic data. Production recovery also requires the matching `AI_CONFIG_ENCRYPTION_KEY`, any retained legacy files under `PLUGIN_ROOT`, and deployment configuration. Keep those separately in encrypted backup storage. Restore to an isolated environment using the recovery settings below until the recovered state has been checked. Ink 1.0 preserves legacy plugin data and files without executing plugins or their schedules.

The encrypted-secret and trusted-plugin recovery check below also passed on 2026-10-08. Its random keys and values are synthetic and never come from deployment files or environment credentials.

## Reproduce the database check

Run from the repository root with Docker running and the Go version specified by `server/go.mod`. The commands create a disposable container, use only synthetic fixture data, and delete the dump and test container on exit. They do not run the API or contact Memobird.

The release gate runs the equivalent `bash scripts/check-backup-restore.sh`, including the current migration files, all public table comparisons, and failure-producing assertions below.

```bash
set -eu
umask 077
backup_container="ink-backup-check-$$"
backup_dir=$(mktemp -d)
trap 'docker stop "$backup_container" >/dev/null 2>&1 || true; rm -rf "$backup_dir"' EXIT

docker run --rm --detach --name "$backup_container" \
  --env POSTGRES_PASSWORD=postgres --publish 127.0.0.1::5432 postgres:16 >/dev/null
attempt=0
until docker exec "$backup_container" pg_isready -h 127.0.0.1 -U postgres >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  test "$attempt" -lt 30
  sleep 1
done
backup_address=$(docker port "$backup_container" 5432/tcp)
backup_port=${backup_address##*:}

INK_TEST_DATABASE_URL="postgres://postgres:postgres@127.0.0.1:$backup_port/postgres?sslmode=disable" \
  go -C server test ./internal/platform/store/postgres -count=1

docker exec "$backup_container" createdb -U postgres backup_source
docker exec "$backup_container" createdb -U postgres backup_restored
DATABASE_URL="postgres://postgres:postgres@127.0.0.1:$backup_port/backup_source?sslmode=disable" \
  go -C server run ./cmd/migrate up
docker exec -i "$backup_container" psql -U postgres -d backup_source -v ON_ERROR_STOP=1 \
  < server/testdata/printer_history.sql
docker exec -i "$backup_container" psql -U postgres -d backup_source -v ON_ERROR_STOP=1 <<'SQL'
update printer_bindings set status = 'offline' where id = 'remove-device';
update print_schedules set enabled = false, lease_until = null where id = 'remove-schedule';
insert into workspace_snapshots (user_id, state, revision)
values ('remove-user', '{"localePreference":"zh-CN","preferences":{"confirmBeforeSending":true}}', 7);
SQL

docker exec "$backup_container" pg_dump -U postgres --format=custom --no-owner --no-acl \
  backup_source > "$backup_dir/database.dump"
docker exec -i "$backup_container" pg_restore -U postgres --dbname=backup_restored \
  --no-owner --no-acl --exit-on-error < "$backup_dir/database.dump"

backup_tables=$(docker exec "$backup_container" psql -U postgres -d backup_source -Atqc \
  "select tablename from pg_tables where schemaname = 'public' order by tablename")
for backup_table in $backup_tables; do
  backup_query="select coalesce(jsonb_agg(row_data order by row_data::text), '[]'::jsonb)::text from (select to_jsonb(t) as row_data from $backup_table t) rows"
  backup_original=$(docker exec "$backup_container" psql -U postgres -d backup_source -Atqc "$backup_query")
  backup_restored=$(docker exec "$backup_container" psql -U postgres -d backup_restored -Atqc "$backup_query")
  test "$backup_original" = "$backup_restored"
done

set -- server/migrations/*.sql
backup_migration_count=$#
docker exec -i "$backup_container" psql -U postgres -d backup_restored -At \
  -v ON_ERROR_STOP=1 -v expected_migrations="$backup_migration_count" <<'SQL'
create function pg_temp.assert_recovery(valid boolean, message text) returns void
language plpgsql as $$ begin
  if valid is distinct from true then raise exception '%', message; end if;
end $$;
select pg_temp.assert_recovery(
  (select count(*) = 1 and bool_and(revision = 7 and state->>'localePreference' = 'zh-CN') from workspace_snapshots),
  'workspace recovery mismatch');
select pg_temp.assert_recovery(
  (select count(*) = 3 and count(*) filter (where content <> '') = 3 from print_jobs),
  'print history recovery mismatch');
select pg_temp.assert_recovery(
  (select count(*) = 1 and bool_and(status = 'offline') from printer_bindings),
  'printer binding recovery mismatch');
select pg_temp.assert_recovery(
  (select count(*) = 1 and bool_and(not enabled and lease_until is null) from print_schedules),
  'legacy schedule recovery mismatch');
select pg_temp.assert_recovery((select count(*) = 1 from print_schedule_deliveries),
  'legacy delivery history recovery mismatch');
select pg_temp.assert_recovery((select count(*) = :expected_migrations from schema_migrations),
  'migration recovery mismatch');
SQL
```

The final assertions raise a PostgreSQL error and stop the script on any mismatch; migration count is taken from the current SQL files. The preceding comparison must succeed for every public table. This procedure assumes Bash or a POSIX shell; in zsh, run it inside `bash` so the table list is split into individual names.

The printer integration test verifies that removal preserves printed, queued, and pending jobs and rejects another user's removal. Accepted queued jobs remain eligible for background completion checks. Rebinding reuses the retained binding. Legacy plugin schedules remain inactive in 1.0.

## Legacy encrypted secrets and trusted plugin files

This test checks encrypted AI configuration and retained legacy data/files without executing plugins. It starts and removes its own PostgreSQL 16 container, creates source and restored databases, and generates a random 32-byte encryption key in memory. It does not read `.env`, `DATABASE_URL`, or `AI_CONFIG_ENCRYPTION_KEY`. Docker must be running.

```bash
INK_TEST_BACKUP_RECOVERY=1 go -C server test -race ./internal/platform/store/postgres \
  -run TestBackupRestoresEncryptedSecretsAndRetainedFiles -count=1 -v
go -C server test -race ./internal/platform/secret
```

The test writes an AI provider key through its service and a synthetic retained plugin secret using AES-GCM, then runs a real `pg_dump` and `pg_restore`. It checks recovered ciphertext and nonce bytes and uses the original key to restore the values. Wrong or missing keys and damaged nonce data fail without plaintext. Public summaries and failure messages are checked for synthetic secret disclosure.

For retained plugin files, it copies the repository fixture to a temporary installation, backs up the directory, removes the source, and restores it to the recorded database path. It compares restored file bytes without executing a plugin or dependency installer. AI uses a local test completion client to check the recovered configuration; it does not contact a model provider. No physical printer or external source is contacted.

Ordinary Go test runs skip this Docker recovery test unless `INK_TEST_BACKUP_RECOVERY=1` is set. The focused secret unit tests always run and cover wrong keys, missing or truncated nonce data, modified ciphertext, and missing key configuration.

## Safe recovery API checks

Restore into a separate database and working directory with no production traffic. Before starting the API, explicitly override any copied deployment settings:

```dotenv
WORKERS_ENABLED=false
PRINT_STATUS_SYNC_ENABLED=false
MEMOBIRD_ACCESS_KEY=
METRICS_ADDR=
OTEL_EXPORTER_OTLP_ENDPOINT=
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=
```

`WORKERS_ENABLED=false` prevents background print-status synchronization. Legacy plugins, fetchers, schedule delivery and inbox cleanup are never started by Ink 1.0. Readiness still checks the database, so `/readyz` can be 200 with workers intentionally disabled. This is a recovery inspection mode, not normal printing.

Use host/container network rules to deny outbound internet access from the recovery API and allow only its isolated PostgreSQL endpoint and local test client. This also prevents accidental use of recovered AI provider credentials. The worker switch does not disable manual API operations: do not invoke printer binding, create/submit/retry printing, or AI generation during inspection. Check health/readiness, login, workspace reads, existing printer/job lists and PNG preview only. Preview neither creates a job nor contacts Memobird.

Keep legacy plugin files at their recorded paths in a private archival backup if retained data needs recovery. Preserve the matching encryption key without printing it in logs. After checking the isolated copy, make a separate deliberate cutover to the intended database and settings; enable workers only there. Avoid running live and recovered copies with the same provider credentials.

## Production recovery boundary

These experiments do not establish a production backup frequency, retention period, RPO, or RTO. The file and encryption checks use a dependency-free trusted fixture and synthetic secrets; they do not establish compatibility of a deployment's plugin dependencies, runtime versions, operating system, real encryption keys, or physical printer. Define those operational requirements for the actual deployment and repeat recovery against an isolated copy of representative data before relying on the backup. Run migrations forward during upgrades; restoring a dump is a recovery action and can discard writes made since that backup.
