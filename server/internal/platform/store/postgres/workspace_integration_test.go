package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ruhuang/ink/server/internal/workspace"
)

func TestWorkspaceSnapshotExcludesPrintHistory(t *testing.T) {
	db := workspaceTestDatabase(t)
	ctx := t.Context()
	_, err := db.Exec(ctx, `
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
	if loaded == nil || loaded.Revision != 1 || len(loaded.PrintJobs) != 0 || len(loaded.Conversations) != 1 {
		t.Fatalf("unexpected workspace projection: %+v", loaded)
	}
	loaded.PrintJobs = []workspace.PrintJob{{ID: "attempted-duplicate", Content: "do not persist"}}
	if _, err := store.SaveByUserID(ctx, "snapshot-user", *loaded, time.Now()); err != nil {
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

func TestWorkspaceConcurrentSavesRejectStaleRevision(t *testing.T) {
	db := workspaceTestDatabase(t)
	ctx := t.Context()
	if _, err := db.Exec(ctx, `insert into users (id,email,password_hash,display_name,status)
		values ('revision-user','revision@example.com','hash','Revision','active')`); err != nil {
		t.Fatal(err)
	}
	store := New(db)
	if err := store.InitializeByUserID(ctx, "revision-user", workspace.EmptyState(), time.Now()); err != nil {
		t.Fatal(err)
	}
	initial, err := store.FindByUserID(ctx, "revision-user")
	if err != nil || initial == nil || initial.Revision != 1 {
		t.Fatalf("initial snapshot: %+v, %v", initial, err)
	}
	type result struct {
		conversation string
		revision     int64
		err          error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for _, id := range []string{"first-tab", "second-tab"} {
		workers.Go(func() {
			<-start
			state := *initial
			state.Conversations = []workspace.Conversation{{ID: id, Title: id}}
			revision, err := store.SaveByUserID(ctx, "revision-user", state, time.Now())
			results <- result{conversation: id, revision: revision, err: err}
		})
	}
	close(start)
	workers.Wait()
	close(results)
	var winner string
	conflicts := 0
	for item := range results {
		switch {
		case item.err == nil:
			if winner != "" || item.revision != 2 {
				t.Fatalf("expected exactly one revision 2 save, got %+v", item)
			}
			winner = item.conversation
		case errors.Is(item.err, workspace.ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent save error: %v", item.err)
		}
	}
	if winner == "" || conflicts != 1 {
		t.Fatalf("winner=%q conflicts=%d", winner, conflicts)
	}
	// A GET that first observed no row may initialize after a different client saved.
	if err := store.InitializeByUserID(ctx, "revision-user", workspace.EmptyState(), time.Now()); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.FindByUserID(ctx, "revision-user")
	if err != nil || loaded == nil || loaded.Revision != 2 || len(loaded.Conversations) != 1 || loaded.Conversations[0].ID != winner {
		t.Fatalf("initialization or stale save replaced the winner: %+v, %v", loaded, err)
	}
	if _, err := store.SaveByUserID(ctx, "revision-user", workspace.EmptyState(), time.Now()); !errors.Is(err, workspace.ErrRevisionRequired) {
		t.Fatalf("expected unversioned save rejection, got %v", err)
	}
}

func workspaceTestDatabase(t *testing.T) *pgxpool.Pool {
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
	for version := 1; version <= 10; version++ {
		applyMigration(t, ctx, db, version)
	}
	applyMigration(t, ctx, db, 10)
	return db
}
