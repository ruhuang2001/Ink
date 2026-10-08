# Backup and restore verification

On 2026-10-08, a PostgreSQL 16 custom-format dump was restored into a second, empty test database. All 13 public tables matched, including migrations 001–010, workspace revision 7, three complete print bodies, an offline printer binding, its disabled schedule, and delivery history. No development or production database was used.

This checks database recovery with synthetic data. Production recovery also requires the matching `AI_CONFIG_ENCRYPTION_KEY`, installed files under `PLUGIN_ROOT`, and deployment configuration. Keep those separately in encrypted backup storage. Restore to an isolated environment with automatic workers and physical printing disabled until the recovered state has been checked.

The encrypted-secret and trusted-plugin recovery check below also passed on 2026-10-08. Its random keys and values are synthetic and never come from deployment files or environment credentials.

## Reproduce the database check

Run from the repository root with Docker running and the Go version specified by `server/go.mod`. The commands create a disposable container, use only synthetic fixture data, and delete the dump and test container on exit. They do not run the API or contact Memobird.

```bash
set -eu
umask 077
backup_container="ink-backup-check-$$"
backup_dir=$(mktemp -d)
trap 'docker stop "$backup_container" >/dev/null 2>&1 || true; rm -rf "$backup_dir"' EXIT

docker run --rm --detach --name "$backup_container" \
  --env POSTGRES_PASSWORD=postgres --publish 127.0.0.1::5432 postgres:16 >/dev/null
attempt=0
until docker exec "$backup_container" pg_isready -U postgres >/dev/null 2>&1; do
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

docker exec -i "$backup_container" psql -U postgres -d backup_restored -At -v ON_ERROR_STOP=1 <<'SQL'
select revision = 7 and state->>'localePreference' = 'zh-CN' from workspace_snapshots;
select count(*) = 3 and count(*) filter (where content <> '') = 3 from print_jobs;
select status = 'offline' from printer_bindings;
select not enabled and lease_until is null from print_schedules;
select count(*) = 1 from print_schedule_deliveries;
select count(*) = 10 from schema_migrations;
SQL
```

The final queries return six `t` values. The preceding comparison must succeed for every public table. This procedure assumes Bash or a POSIX shell; in zsh, run it inside `bash` so the table list is split into individual names.

The printer integration test also verifies that removal preserves printed, queued, and pending jobs, rejects another user's removal, stops schedule claims, and prevents a stale runner from re-enabling the schedule. Accepted queued jobs remain eligible for background completion checks. Rebinding reuses the retained binding; the schedule must be enabled explicitly.

## Encrypted secrets and trusted plugin files

The opt-in recovery integration test starts and removes its own PostgreSQL 16 container, creates source and restored databases, and generates a random 32-byte encryption key in memory. It does not read `.env`, `DATABASE_URL`, or `AI_CONFIG_ENCRYPTION_KEY`. Docker must be running, and `python3` must be available.

```bash
INK_TEST_BACKUP_RECOVERY=1 go -C server test -race ./internal/platform/store/postgres \
  -run TestBackupRestoresEncryptedSecretsAndPluginFiles -count=1 -v
go -C server test -race ./internal/platform/secret
```

The test writes an AI provider key and a plugin binding secret through the existing services using AES-GCM, then runs a real `pg_dump` and `pg_restore`. It checks that recovered ciphertext and nonce bytes are identical and that the original key restores the same values. Wrong or missing keys prevent AI/plugin use and return no plaintext; damaged nonce data returns an error instead of panicking. Public summaries and failure messages are checked for synthetic secret disclosure.

For plugin files, it copies the trusted repository Python fixture to a temporary installation, backs up that directory, removes the source, and restores it to the path recorded in the recovered database. The real plugin subprocess fetch then produces the same structured result as before recovery. AI uses a local test completion client to check the recovered configuration; it does not contact a model provider. The fixture has no third-party runtime dependencies, and no physical printer or external source is contacted.

Ordinary Go test runs skip this Docker recovery test unless `INK_TEST_BACKUP_RECOVERY=1` is set. The focused secret unit tests always run and cover wrong keys, missing or truncated nonce data, modified ciphertext, and missing key configuration.

## Production recovery boundary

These experiments do not establish a production backup frequency, retention period, RPO, or RTO. The file and encryption checks use a dependency-free trusted fixture and synthetic secrets; they do not establish compatibility of a deployment's plugin dependencies, runtime versions, operating system, real encryption keys, or physical printer. Define those operational requirements for the actual deployment and repeat recovery against an isolated copy of representative data before relying on the backup. Run migrations forward during upgrades; restoring a dump is a recovery action and can discard writes made since that backup.
