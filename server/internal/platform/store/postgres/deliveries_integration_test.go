package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ruhuang/ink/server/internal/dispatch"
)

func TestDeliveryMigrationUpgradeAndClaims(t *testing.T) {
	databaseURL := os.Getenv("INK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INK_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(admin.Close)
	if err := admin.Ping(ctx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	schema := fmt.Sprintf("delivery_upgrade_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "create schema "+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "drop schema "+schema+" cascade"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse PostgreSQL URL: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect to test schema: %v", err)
	}
	t.Cleanup(db.Close)

	for version := 1; version <= 7; version++ {
		applyMigration(t, ctx, db, version)
	}
	insertDeliveryUpgradeFixture(t, ctx, db)
	applyMigration(t, ctx, db, 8)

	assertLegacyDeliveriesPreserved(t, ctx, db)
	assertDeliveryMigrationShape(t, ctx, db)
	assertAtomicDeliveryClaims(t, ctx, db)
}

func applyMigration(t *testing.T, ctx context.Context, db *pgxpool.Pool, version int) {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	pattern := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "..", "migrations", fmt.Sprintf("%03d_*.sql", version))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("find migration %03d: %v", version, err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one migration for %03d, found %d", version, len(matches))
	}
	sql, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read migration %03d: %v", version, err)
	}
	if _, err := db.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("apply migration %03d: %v", version, err)
	}
}

func insertDeliveryUpgradeFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	_, err := db.Exec(ctx, `
		insert into users (id, email, password_hash, display_name, status)
		values ('user_upgrade', 'upgrade@example.com', 'hash', 'Upgrade', 'active');

		insert into printer_bindings (
			id, user_id, name, device_identifier, provider_user_id, status, created_at, updated_at
		) values ('printer_upgrade', 'user_upgrade', 'Printer', 'device-upgrade', 1, 'connected', now(), now());

		insert into plugin_installations (
			id, plugin_key, source_type, display_name, version, runtime_type, manifest_json,
			current_path, status, created_at, updated_at
		) values (
			'installation_upgrade', 'upgrade.plugin', 'upload', 'Upgrade plugin', '1.0.0', 'node',
			'{}'::jsonb, '/tmp/plugin', 'ready', now(), now()
		);

		insert into plugin_bindings (
			id, plugin_installation_id, user_id, enabled, status, created_at, updated_at
		) values ('binding_upgrade', 'installation_upgrade', 'user_upgrade', true, 'connected', now(), now());

		insert into print_schedules (
			id, user_id, plugin_installation_id, plugin_binding_id, title, frequency_type,
			timezone, hour, minute, device_id, next_run_at, created_at, updated_at
		) values (
			'schedule_upgrade', 'user_upgrade', 'installation_upgrade', 'binding_upgrade',
			'Upgrade schedule', 'daily', 'UTC', 8, 0, 'printer_upgrade', now(), now(), now()
		);

		insert into plugin_items (
			id, user_id, plugin_installation_id, plugin_binding_id, external_id, title,
			source_label, blocks_json, status, fetched_at, created_at, updated_at
		) values
			('item_printed', 'user_upgrade', 'installation_upgrade', 'binding_upgrade', 'printed',
			 'Printed', 'Fixture', '[]'::jsonb, 'pending', now(), now(), now()),
			('item_failed', 'user_upgrade', 'installation_upgrade', 'binding_upgrade', 'failed',
			 'Failed', 'Fixture', '[]'::jsonb, 'pending', now(), now(), now()),
			('item_claim', 'user_upgrade', 'installation_upgrade', 'binding_upgrade', 'claim',
			 'Claim', 'Fixture', '[]'::jsonb, 'pending', now(), now(), now()),
			('item_exhausted', 'user_upgrade', 'installation_upgrade', 'binding_upgrade', 'exhausted',
			 'Exhausted', 'Fixture', '[]'::jsonb, 'pending', now(), now(), now());

		insert into print_schedule_deliveries (
			id, print_schedule_id, plugin_item_id, status, attempt_count, last_error, created_at, updated_at
		) values
			('delivery_printed', 'schedule_upgrade', 'item_printed', 'printed', 1, null, now(), now()),
			('delivery_failed', 'schedule_upgrade', 'item_failed', 'failed', 1, 'temporary', now(), now()),
			('delivery_exhausted', 'schedule_upgrade', 'item_exhausted', 'failed', 3, 'permanent', now(), now());
	`)
	if err != nil {
		t.Fatalf("insert migration 007 fixture: %v", err)
	}
}

func assertLegacyDeliveriesPreserved(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	rows, err := db.Query(ctx, `
		select id, status, attempt_count, last_error, lease_until
		from print_schedule_deliveries
		where id in ('delivery_printed', 'delivery_failed')
		order by id
	`)
	if err != nil {
		t.Fatalf("read upgraded deliveries: %v", err)
	}
	defer rows.Close()

	type legacyDelivery struct {
		id           string
		status       string
		attemptCount int
		lastError    *string
		leaseUntil   *time.Time
	}
	want := []legacyDelivery{
		{id: "delivery_failed", status: "failed", attemptCount: 1, lastError: stringPointer("temporary")},
		{id: "delivery_printed", status: "printed", attemptCount: 1},
	}
	var got []legacyDelivery
	for rows.Next() {
		var current legacyDelivery
		if err := rows.Scan(&current.id, &current.status, &current.attemptCount, &current.lastError, &current.leaseUntil); err != nil {
			t.Fatalf("scan upgraded delivery: %v", err)
		}
		got = append(got, current)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate upgraded deliveries: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy deliveries changed during upgrade: got %+v, want %+v", got, want)
	}
}

func assertDeliveryMigrationShape(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	var leaseColumnCount int
	if err := db.QueryRow(ctx, `
		select count(*)
		from information_schema.columns
		where table_schema = current_schema()
		  and table_name = 'print_schedule_deliveries'
		  and column_name = 'lease_until'
	`).Scan(&leaseColumnCount); err != nil {
		t.Fatalf("inspect lease column: %v", err)
	}
	if leaseColumnCount != 1 {
		t.Fatalf("lease_until column count = %d, want 1", leaseColumnCount)
	}

	var indexExists bool
	if err := db.QueryRow(ctx, `
		select exists (
			select 1 from pg_indexes
			where schemaname = current_schema()
			  and tablename = 'print_schedule_deliveries'
			  and indexname = 'print_schedule_deliveries_retryable_idx'
		)
	`).Scan(&indexExists); err != nil {
		t.Fatalf("inspect retryable index: %v", err)
	}
	if !indexExists {
		t.Fatal("retryable delivery index was not created")
	}
}

func assertAtomicDeliveryClaims(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	store := New(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	delivery := dispatch.Delivery{
		ID:              "delivery_claim",
		PrintScheduleID: "schedule_upgrade",
		PluginItemID:    "item_claim",
		CreatedAt:       now,
	}

	const contenders = 12
	start := make(chan struct{})
	var claimedCount atomic.Int32
	errCh := make(chan error, contenders)
	var wait sync.WaitGroup
	for range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, claimed, err := store.ClaimDelivery(ctx, delivery, now.Add(dispatch.ReservationLease))
			if err != nil {
				errCh <- err
				return
			}
			if claimed {
				claimedCount.Add(1)
			}
		}()
	}
	close(start)
	wait.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent claim: %v", err)
	}
	if got := claimedCount.Load(); got != 1 {
		t.Fatalf("successful concurrent claims = %d, want 1", got)
	}

	if _, claimed, err := store.ClaimDelivery(ctx, delivery, now.Add(dispatch.ReservationLease)); err != nil {
		t.Fatalf("claim active reservation: %v", err)
	} else if claimed {
		t.Fatal("active reservation was claimed twice")
	}

	if _, err := db.Exec(ctx, `
		update print_schedule_deliveries
		set lease_until = $2
		where id = $1
	`, delivery.ID, now.Add(-time.Second)); err != nil {
		t.Fatalf("expire reservation: %v", err)
	}
	reclaimed, claimed, err := store.ClaimDelivery(ctx, delivery, now.Add(dispatch.ReservationLease))
	if err != nil {
		t.Fatalf("reclaim expired reservation: %v", err)
	}
	if !claimed || reclaimed.AttemptCount != 2 {
		t.Fatalf("expired reservation claim = (%v, attempt %d), want (true, attempt 2)", claimed, reclaimed.AttemptCount)
	}

	failed := delivery
	failed.ID = "delivery_failed"
	failed.PluginItemID = "item_failed"
	retried, claimed, err := store.ClaimDelivery(ctx, failed, now.Add(dispatch.ReservationLease))
	if err != nil {
		t.Fatalf("claim failed delivery: %v", err)
	}
	if !claimed || retried.AttemptCount != 2 {
		t.Fatalf("failed delivery claim = (%v, attempt %d), want (true, attempt 2)", claimed, retried.AttemptCount)
	}

	exhausted := delivery
	exhausted.ID = "delivery_exhausted"
	exhausted.PluginItemID = "item_exhausted"
	if _, claimed, err := store.ClaimDelivery(ctx, exhausted, now.Add(dispatch.ReservationLease)); err != nil {
		t.Fatalf("claim exhausted delivery: %v", err)
	} else if claimed {
		t.Fatal("delivery at the attempt limit was claimed")
	}
}

func stringPointer(value string) *string {
	return &value
}
