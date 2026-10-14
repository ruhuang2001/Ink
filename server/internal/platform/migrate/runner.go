package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runner applies SQL migrations in filename order.
type Runner struct {
	db *pgxpool.Pool
}

// NewRunner constructs a migration runner for the provided database pool.
func NewRunner(db *pgxpool.Pool) *Runner {
	return &Runner{db: db}
}

// Up applies every migration file that has not been recorded yet.
func (r *Runner) Up(ctx context.Context, dir string) ([]string, error) {
	files, err := migrationFiles(dir)
	if err != nil {
		return nil, err
	}
	conn, err := r.db.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	const lockID int64 = 5282242231001
	if _, err := conn.Exec(ctx, `select pg_advisory_lock($1)`, lockID); err != nil {
		// A canceled response cannot prove the server did not acquire this
		// session lock. Never return that session to the pool.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = conn.Conn().Close(cleanupCtx)
		return nil, fmt.Errorf("lock migrations: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(cleanupCtx, `select pg_advisory_unlock($1)`, lockID); err != nil {
			_ = conn.Conn().Close(cleanupCtx)
		}
	}()
	if err := ensureSchemaMigrationsTable(ctx, conn); err != nil {
		return nil, err
	}

	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return nil, err
	}

	checksums := make(map[string]string, len(files))
	sources := make(map[string][]byte, len(files))
	for _, file := range files {
		sql, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", filepath.Base(file), err)
		}
		sum := sha256.Sum256(sql)
		version := filepath.Base(file)
		checksums[version] = hex.EncodeToString(sum[:])
		sources[version] = sql
	}
	for version, checksum := range applied {
		actual, exists := checksums[version]
		if !exists {
			return nil, fmt.Errorf("applied migration %s is missing from source", version)
		}
		if checksum != nil && *checksum != actual {
			return nil, fmt.Errorf("applied migration %s checksum mismatch; restore its original SQL and add a new migration", version)
		}
	}
	// Pre-1.0 records adopt a checksum once. Verify the checkout before upgrading.
	for version, checksum := range applied {
		if checksum == nil {
			if _, err := conn.Exec(ctx, `update schema_migrations set checksum=$2 where version=$1 and checksum is null`, version, checksums[version]); err != nil {
				return nil, err
			}
		}
	}

	appliedNow := make([]string, 0)

	for _, file := range files {
		version := filepath.Base(file)
		if _, exists := applied[version]; exists {
			continue
		}

		if err := applyFile(ctx, conn, sources[version], version, checksums[version]); err != nil {
			return appliedNow, err
		}

		appliedNow = append(appliedNow, version)
	}

	return appliedNow, nil
}

func ensureSchemaMigrationsTable(ctx context.Context, conn *pgxpool.Conn) error {
	_, err := conn.Exec(ctx, `
		create table if not exists schema_migrations (
			version text primary key,
			applied_at timestamptz not null,
			checksum text
		);
		alter table schema_migrations add column if not exists checksum text
	`)
	return err
}

func appliedVersions(ctx context.Context, conn *pgxpool.Conn) (map[string]*string, error) {
	rows, err := conn.Query(ctx, `select version, checksum from schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := make(map[string]*string)
	for rows.Next() {
		var version string
		var checksum *string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, err
		}
		versions[version] = checksum
	}

	return versions, rows.Err()
}

func applyFile(ctx context.Context, conn *pgxpool.Conn, sqlBytes []byte, version, checksum string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}

	if _, err := tx.Exec(ctx, `
		insert into schema_migrations (version, applied_at, checksum)
		values ($1, $2, $3)
	`, version, time.Now().UTC(), checksum); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}

	return tx.Commit(ctx)
}

func migrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %s: %w", dir, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		files = append(files, filepath.Join(dir, entry.Name()))
	}

	slices.Sort(files)
	return files, nil
}
