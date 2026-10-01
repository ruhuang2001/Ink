package postgres

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/workspace"
)

func TestPrintJobsMigrationPaginationAndStatusCAS(t *testing.T) {
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
	schema := fmt.Sprintf("print_sync_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "create schema "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "drop schema "+schema+" cascade"); err != nil {
			t.Error(err)
		}
	})
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	for version := 1; version <= 8; version++ {
		applyMigration(t, ctx, db, version)
	}
	_, err = db.Exec(ctx, `
		insert into users (id,email,password_hash,display_name,status)
		values ('print-user','print@example.com','hash','Print','active'), ('other-user','other-print@example.com','hash','Other','active');
		insert into printer_bindings (id,user_id,name,device_identifier,provider_user_id,status,created_at,updated_at)
		values ('print-device','print-user','Printer','device',1,'connected',now(),now()), ('other-device','other-user','Other','device',1,'connected',now(),now());
		insert into print_jobs (id,user_id,printer_binding_id,title,source,content,status,provider_print_content_id,created_at,updated_at)
		values ('legacy-queued','print-user','print-device','Legacy','Manual','large old body','queued',1,now(),now());
	`)
	if err != nil {
		t.Fatal(err)
	}
	applyMigration(t, ctx, db, 9)
	store := New(db)
	legacy, err := store.FindJobByID(ctx, "print-user", "legacy-queued")
	if err != nil || legacy.NextStatusCheckAt == nil || legacy.Content != "large old body" {
		t.Fatalf("migration failed to schedule legacy job: %+v, %v", legacy, err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for index := range 8 {
		job := printer.Job{ID: fmt.Sprintf("page-%d", index), UserID: "print-user", PrinterBindingID: "print-device", Title: "Row", Source: "Manual", Content: "keep full body", Status: workspace.PrintStatusQueued, ProviderPrintContentID: new(10 + index), CreatedAt: now.Add(time.Hour), UpdatedAt: now, NextStatusCheckAt: new(now), StatusCheckAttempts: 1}
		if err := store.SaveJob(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	foreign := printer.Job{ID: "foreign", UserID: "other-user", PrinterBindingID: "other-device", Title: "Foreign", Source: "Manual", Content: "private", Status: workspace.PrintStatusQueued, ProviderPrintContentID: new(111), CreatedAt: now.Add(2 * time.Hour), UpdatedAt: now, NextStatusCheckAt: new(now.Add(time.Hour))}
	if err := store.SaveJob(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	first, err := store.ListJobSummaries(ctx, "print-user", printer.JobPageQuery{Status: "active", Limit: 4})
	if err != nil || len(first) != 4 || first[0].ID != "page-7" {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	// Updating a row below the cursor must not remove it from the next page.
	if _, err := db.Exec(ctx, "update print_jobs set updated_at = $1 where id = 'page-1'", now.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	last := first[len(first)-1]
	second, err := store.ListJobSummaries(ctx, "print-user", printer.JobPageQuery{Status: "active", Limit: 10, Cursor: &printer.JobCursor{ID: last.ID, CreatedAt: last.CreatedAt}})
	if err != nil || len(second) != 5 || second[0].ID != "page-3" || second[4].ID != "legacy-queued" {
		t.Fatalf("stable second page: %+v, %v", second, err)
	}
	statuses, err := store.ListJobStatuses(ctx, "print-user", []string{"page-7", "foreign"})
	if err != nil || len(statuses) != 1 || statuses[0].ID != "page-7" {
		t.Fatalf("status account isolation: %+v, %v", statuses, err)
	}
	counts, latest, err := store.GetJobCounts(ctx, "print-user", now.Add(-time.Hour))
	if err != nil || counts.Queued != 9 || latest == nil || *latest != "page-7" {
		t.Fatalf("global counts: %+v latest=%v err=%v", counts, latest, err)
	}
	pending := printer.Job{ID: "pending-cas", UserID: "print-user", PrinterBindingID: "print-device", Title: "Pending", Source: "Manual", Content: "body", Status: workspace.PrintStatusPending, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveJob(ctx, pending); err != nil {
		t.Fatal(err)
	}
	claimed := pending
	claimed.Status = workspace.PrintStatusQueued
	claimed.UpdatedAt = now.Add(time.Microsecond)
	if changed, err := store.SavePendingJob(ctx, claimed, pending.UpdatedAt); err != nil || !changed {
		t.Fatalf("claim pending job: %v, %v", changed, err)
	}
	if changed, err := store.SavePendingJob(ctx, claimed, pending.UpdatedAt); err != nil || changed {
		t.Fatalf("stale pending mutation should fail: %v, %v", changed, err)
	}

	jobs, err := store.ListDueStatusJobs(ctx, now.Add(time.Second), 3)
	if err != nil || len(jobs) != 3 {
		t.Fatalf("due batch: %+v, %v", jobs, err)
	}
	result := printer.StatusCheckResult{CheckedAt: now.Add(time.Second), NextCheck: now.Add(10 * time.Second), Attempts: 0}
	checked := jobs[0]
	updated, err := store.SaveStatusCheck(ctx, checked, result)
	if err != nil || !updated {
		t.Fatalf("reschedule due: %v, %v", updated, err)
	}
	if updated, err := store.SaveStatusCheck(ctx, checked, result); err != nil || updated {
		t.Fatalf("stale due version should fail: %v, %v", updated, err)
	}
	next, err := store.ListDueStatusJobs(ctx, now.Add(time.Second), 20)
	if err != nil || slices.ContainsFunc(next, func(j printer.StatusSyncJob) bool { return j.ID == checked.ID || j.ID == "foreign" }) {
		t.Fatalf("rescheduled or foreign not-due job present: %+v, %v", next, err)
	}
	for _, test := range []struct{ name, update string }{
		{"provider retry", "provider_print_content_id = provider_print_content_id + 1"},
		{"same provider newer version", "updated_at = updated_at + interval '1 second'"},
		{"terminal", "status = 'cancelled'"},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidates, err := store.ListDueStatusJobs(t.Context(), now.Add(time.Second), 20)
			if err != nil || len(candidates) == 0 {
				t.Fatal(err)
			}
			candidate := candidates[0]
			if _, err := db.Exec(t.Context(), "update print_jobs set "+test.update+" where id = $1", candidate.ID); err != nil {
				t.Fatal(err)
			}
			result.Completed = true
			if changed, err := store.SaveStatusCheck(t.Context(), candidate, result); err != nil || changed {
				t.Fatalf("stale provider result overwrote %s: %v,%v", test.name, changed, err)
			}
		})
	}
	candidates, err := store.ListDueStatusJobs(ctx, now.Add(time.Second), 20)
	if err != nil || len(candidates) == 0 {
		t.Fatal(err)
	}
	result.Completed = true
	candidate := candidates[0]
	if changed, err := store.SaveStatusCheck(ctx, candidate, result); err != nil || !changed {
		t.Fatalf("completion: %v,%v", changed, err)
	}
	completed, err := store.FindJobByID(ctx, "print-user", candidate.ID)
	if err != nil || completed.Status != workspace.PrintStatusCompleted || completed.NextStatusCheckAt != nil || completed.Content != "keep full body" && completed.ID != "legacy-queued" {
		t.Fatalf("completion overwrote body: %+v,%v", completed, err)
	}
}
