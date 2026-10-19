package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newIsolatedTestDB(t *testing.T, prefix string) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("INK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INK_TEST_DATABASE_URL is not set")
	}
	ctx := t.Context()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "create schema "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanupCtx, "drop schema "+identifier+" cascade"); err != nil {
			t.Error(err)
		}
	})
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}

func applyMigration(t *testing.T, ctx context.Context, db *pgxpool.Pool, version int) {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	pattern := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "..", "migrations", fmt.Sprintf("%03d_*.sql", version))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) != 1 {
		t.Fatalf("find migration %03d: count=%d error=%v", version, len(matches), err)
	}
	sql, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read migration %03d: %v", version, err)
	}
	if _, err := db.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("apply migration %03d: %v", version, err)
	}
}
