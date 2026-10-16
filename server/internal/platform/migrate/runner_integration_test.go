package migrate

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func migrationTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("INK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INK_TEST_DATABASE_URL is not set")
	}
	ctx := t.Context()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal("create migration test administration pool")
	}
	t.Cleanup(admin.Close)
	schema := "migration_test_" + strings.ToLower(rand.Text())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "create schema "+identifier); err != nil {
		t.Fatal("create isolated migration test schema")
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanupCtx, "drop schema "+identifier+" cascade"); err != nil {
			t.Error("remove isolated migration test schema")
		}
	})
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal("parse migration test pool configuration")
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	config.MaxConns = 2
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("create isolated migration pool")
	}
	t.Cleanup(db.Close)
	return db
}

func writeMigration(t *testing.T, dir, name, sql string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(sql), 0o600); err != nil {
		t.Fatal(err)
	}
}

func requireMissingMigrationTable(t *testing.T, db *pgxpool.Pool, name string) {
	t.Helper()
	var exists bool
	if err := db.QueryRow(t.Context(), `select exists(select 1 from information_schema.tables where table_schema=current_schema() and table_name=$1)`, name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("unexpected migration effect: %s exists", name)
	}
}

func TestLegacyMigrationAdoptsChecksumOnce(t *testing.T) {
	db := migrationTestDatabase(t)
	ctx := t.Context()
	dir := t.TempDir()
	sql := "create table legacy_marker (id integer);"
	writeMigration(t, dir, "001.sql", sql)
	if _, err := db.Exec(ctx, `create table schema_migrations(version text primary key, applied_at timestamptz not null); insert into schema_migrations values ('001.sql',now()); create table legacy_marker(id integer)`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		applied, err := NewRunner(db).Up(ctx, dir)
		if err != nil || len(applied) != 0 {
			t.Fatalf("legacy adoption reapplied SQL: applied=%v error=%v", applied, err)
		}
	}
	var checksum string
	if err := db.QueryRow(ctx, "select checksum from schema_migrations where version='001.sql'").Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(sql))
	if checksum != hex.EncodeToString(sum[:]) {
		t.Fatal("legacy migration did not adopt exact source checksum")
	}
	writeMigration(t, dir, "001.sql", sql+" -- changed")
	if _, err := NewRunner(db).Up(ctx, dir); err == nil {
		t.Fatal("adopted checksum was replaced by edited source")
	}
}

func TestMigrationIntegrityFailurePreventsNewSQL(t *testing.T) {
	for _, reason := range []string{"edited", "missing"} {
		t.Run(reason, func(t *testing.T) {
			db := migrationTestDatabase(t)
			dir := t.TempDir()
			writeMigration(t, dir, "001.sql", "create table original_marker(id integer);")
			if _, err := NewRunner(db).Up(t.Context(), dir); err != nil {
				t.Fatal(err)
			}
			if reason == "edited" {
				writeMigration(t, dir, "001.sql", "drop table original_marker;")
			} else if err := os.Remove(filepath.Join(dir, "001.sql")); err != nil {
				t.Fatal(err)
			}
			writeMigration(t, dir, "002.sql", "create table forbidden_marker(id integer);")
			if _, err := NewRunner(db).Up(t.Context(), dir); err == nil {
				t.Fatal("expected migration integrity failure")
			}
			requireMissingMigrationTable(t, db, "forbidden_marker")
			var count int
			if err := db.QueryRow(t.Context(), "select count(*) from schema_migrations").Scan(&count); err != nil || count != 1 {
				t.Fatalf("integrity failure changed history: count=%d error=%v", count, err)
			}
		})
	}
}

func TestFailedMigrationRollsBackSQLAndHistory(t *testing.T) {
	db := migrationTestDatabase(t)
	dir := t.TempDir()
	writeMigration(t, dir, "001.sql", "create table rollback_marker(id integer); insert into rollback_marker values ('invalid');")
	if _, err := NewRunner(db).Up(t.Context(), dir); err == nil {
		t.Fatal("expected failed SQL")
	}
	requireMissingMigrationTable(t, db, "rollback_marker")
	var count int
	if err := db.QueryRow(t.Context(), "select count(*) from schema_migrations").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed migration recorded history: count=%d error=%v", count, err)
	}
	writeMigration(t, dir, "001.sql", "create table rollback_marker(id integer); insert into rollback_marker values (1);")
	if applied, err := NewRunner(db).Up(t.Context(), dir); err != nil || len(applied) != 1 {
		t.Fatalf("failed migration lock/transaction was not released: applied=%v error=%v", applied, err)
	}
}

func TestConcurrentMigrationRunnersApplyOnce(t *testing.T) {
	db := migrationTestDatabase(t)
	dir := t.TempDir()
	writeMigration(t, dir, "001.sql", "create table apply_marker(id integer); insert into apply_marker values(1); select pg_sleep(0.05);")
	type result struct {
		applied []string
		err     error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() { <-start; applied, err := NewRunner(db).Up(t.Context(), dir); results <- result{applied, err} })
	}
	close(start)
	workers.Wait()
	close(results)
	applications := 0
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		applications += len(result.applied)
	}
	var count int
	if err := db.QueryRow(t.Context(), "select count(*) from apply_marker").Scan(&count); err != nil || count != 1 || applications != 1 {
		t.Fatalf("migration was not applied exactly once: applications=%d rows=%d error=%v", applications, count, err)
	}
}

func TestCanceledMigrationLockWaitReleasesConnection(t *testing.T) {
	db := migrationTestDatabase(t)
	dir := t.TempDir()
	writeMigration(t, dir, "001.sql", "create table cancellation_marker(id integer);")
	holder, err := db.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	const lockID int64 = 5282242231001
	if _, err := holder.Exec(t.Context(), "select pg_advisory_lock($1)", lockID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancel()
		_, _ = holder.Exec(cleanupCtx, "select pg_advisory_unlock($1)", lockID)
	}()
	config := db.Config()
	config.MaxConns = 1
	waiter, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer waiter.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := NewRunner(waiter).Up(ctx, dir); err == nil {
		t.Fatal("canceled lock waiter unexpectedly applied migration")
	}
	if ctx.Err() == nil || time.Since(started) > 2*time.Second {
		t.Fatal("lock wait did not respect cancellation")
	}
	// pgxpool destroys canceled connections through puddle's asynchronous
	// destructor. Require bounded release instead of inspecting its transient
	// acquired-resource counter immediately after Release returns.
	releaseCtx, releaseCancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer releaseCancel()
	for waiter.Stat().AcquiredConns() != 0 {
		select {
		case <-releaseCtx.Done():
			t.Fatal("canceled lock waiter retained its pooled connection")
		case <-time.After(time.Millisecond):
		}
	}
	probeCtx, probeCancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer probeCancel()
	if err := waiter.Ping(probeCtx); err != nil {
		t.Fatal("canceled lock waiter left the pool unusable")
	}
	requireMissingMigrationTable(t, db, "cancellation_marker")
	if _, err := holder.Exec(t.Context(), "select pg_advisory_unlock($1)", lockID); err != nil {
		t.Fatal(err)
	}
	var available bool
	if err := holder.QueryRow(probeCtx, "select pg_try_advisory_lock($1)", lockID).Scan(&available); err != nil || !available {
		t.Fatalf("canceled waiter left a session advisory lock: available=%t error=%v", available, err)
	}
	if _, err := holder.Exec(probeCtx, "select pg_advisory_unlock($1)", lockID); err != nil {
		t.Fatal(err)
	}
	if applied, err := NewRunner(waiter).Up(probeCtx, dir); err != nil || len(applied) != 1 {
		t.Fatalf("migration could not resume after canceled lock wait: applied=%v error=%v", applied, err)
	}
}
