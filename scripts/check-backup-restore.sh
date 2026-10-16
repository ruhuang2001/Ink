#!/usr/bin/env bash
# Disposable PostgreSQL recovery; synthetic data only; no API or provider calls.
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
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
echo "Core and retained legacy database recovery assertions passed."
