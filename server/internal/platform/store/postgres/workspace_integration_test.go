package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ruhuang/ink/server/internal/workspace"
)

func TestWorkspaceSnapshotExcludesPrintHistory(t *testing.T) {
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
	schema := fmt.Sprintf("workspace_print_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "create schema "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "drop schema "+schema+" cascade"); err != nil {
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
	for version := 1; version <= 9; version++ {
		applyMigration(t, ctx, db, version)
	}
	_, err = db.Exec(ctx, `
		insert into users (id,email,password_hash,display_name,status)
		values ('snapshot-user','snapshot@example.com','hash','Snapshot','active');
		insert into printer_bindings (id,user_id,name,device_identifier,provider_user_id,status,created_at,updated_at)
		values ('snapshot-printer','snapshot-user','Printer','device',1,'connected',now(),now());
		insert into print_jobs (id,user_id,printer_binding_id,title,source,content,status,created_at,updated_at)
		values ('authoritative-job','snapshot-user','snapshot-printer','Job','Manual','keep full body','completed',now(),now());
		insert into workspace_snapshots (user_id,state)
		values ('snapshot-user','{"printJobs":[{"id":"authoritative-job","content":"duplicated body"}],"conversations":[{"id":"keep-conversation"}]}'::jsonb);
	`)
	if err != nil {
		t.Fatal(err)
	}
	store := New(db)
	loaded, err := store.FindByUserID(ctx, "snapshot-user")
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || len(loaded.PrintJobs) != 0 || len(loaded.Conversations) != 1 {
		t.Fatalf("unexpected workspace projection: %+v", loaded)
	}
	loaded.PrintJobs = []workspace.PrintJob{{ID: "attempted-duplicate", Content: "do not persist"}}
	if err := store.SaveByUserID(ctx, "snapshot-user", *loaded, time.Now()); err != nil {
		t.Fatal(err)
	}
	var duplicateCount int
	if err := db.QueryRow(ctx, "select jsonb_array_length(state->'printJobs') from workspace_snapshots where user_id='snapshot-user'").Scan(&duplicateCount); err != nil {
		t.Fatal(err)
	}
	if duplicateCount != 0 {
		t.Fatal("workspace must not persist copied print bodies")
	}
	job, err := store.FindJobByID(ctx, "snapshot-user", "authoritative-job")
	if err != nil {
		t.Fatal(err)
	}
	if job == nil || job.Content != "keep full body" {
		t.Fatal("workspace save changed authoritative print history")
	}
}
