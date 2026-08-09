package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ruhuang/ink/server/internal/platform/config"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "setup" && os.Args[1] != "cleanup") {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/smoke-fixture setup|cleanup")
		os.Exit(1)
	}
	if err := config.LoadDotEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "load .env: %v\n", err)
		os.Exit(1)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	printerID := os.Getenv("INK_SMOKE_PRINTER_ID")
	pluginKey := os.Getenv("INK_SMOKE_PLUGIN_KEY")
	if databaseURL == "" || printerID == "" || pluginKey == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL, INK_SMOKE_PRINTER_ID, and INK_SMOKE_PLUGIN_KEY are required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if os.Args[1] == "setup" {
		err = setup(ctx, db, printerID)
	} else {
		err = cleanup(ctx, db, printerID, pluginKey)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s smoke fixture: %v\n", os.Args[1], err)
		os.Exit(1)
	}
}

func setup(ctx context.Context, db *pgxpool.Pool, printerID string) error {
	result, err := db.Exec(ctx, `
		insert into printer_bindings (
			id, user_id, name, note, device_identifier, provider_user_id,
			status, created_at, updated_at
		)
		select $1, id, 'Smoke printer', 'Temporary API smoke fixture', $2, 1,
			'connected', now(), now()
		from users
		where role = 'admin' and status = 'active'
		order by created_at asc
		limit 1
	`, printerID, "smoke-device-"+printerID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("active admin user not found")
	}
	return nil
}

func cleanup(ctx context.Context, db *pgxpool.Pool, printerID string, pluginKey string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `delete from print_jobs where printer_binding_id = $1`, printerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from plugin_installations where plugin_key = $1`, pluginKey); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from printer_bindings where id = $1`, printerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
