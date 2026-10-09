package postgres

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/workspace"
)

func TestStatusClaimsExcludeOtherWorkersAndRecoverAfterExpiry(t *testing.T) {
	db := workspaceTestDatabase(t)
	applyMigration(t, t.Context(), db, 11)
	fixture, err := os.ReadFile("../../../../testdata/printer_history.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(t.Context(), string(fixture)); err != nil {
		t.Fatal(err)
	}
	store := New(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	// A legacy writer may insert a queued row without the new due-time column.
	if _, err := db.Exec(t.Context(), "update print_jobs set next_status_check_at = null where id = 'queued-job'"); err != nil {
		t.Fatal(err)
	}
	orphan := printer.Job{ID: "orphan", UserID: "remove-user", PrinterBindingID: "remove-device", Title: "Receipt", Source: "Manual", Content: "keep original body", Status: workspace.PrintStatusQueued, CreatedAt: now.Add(-4 * time.Minute), UpdatedAt: now.Add(-4 * time.Minute)}
	if err := store.SaveJob(t.Context(), orphan); err != nil {
		t.Fatal(err)
	}
	type batch struct {
		jobs []printer.StatusSyncJob
		err  error
	}
	results := make(chan batch, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			<-start
			jobs, err := store.ClaimDueStatusJobs(t.Context(), now, now.Add(time.Minute), 20)
			results <- batch{jobs, err}
		})
	}
	close(start)
	workers.Wait()
	close(results)
	var claimed []printer.StatusSyncJob
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		claimed = append(claimed, result.jobs...)
	}
	if len(claimed) != 1 || claimed[0].ID != "queued-job" {
		t.Fatalf("workers duplicated a provider claim: %+v", claimed)
	}
	recovered, err := store.FindJobByID(t.Context(), orphan.UserID, orphan.ID)
	if err != nil || recovered.Status != workspace.PrintStatusFailed || recovered.Content != orphan.Content || recovered.ErrorMessage == nil || *recovered.ErrorMessage != printer.SubmissionOutcomeUnknown {
		t.Fatalf("abandoned submission was not safely recovered: %+v, %v", recovered, err)
	}
	retryAt := now.Add(time.Minute + time.Microsecond)
	newer, err := store.ClaimDueStatusJobs(t.Context(), retryAt, retryAt.Add(time.Minute), 20)
	if err != nil || len(newer) != 1 {
		t.Fatalf("expired claim not recovered: %+v, %v", newer, err)
	}
	result := printer.StatusCheckResult{Completed: true, CheckedAt: retryAt, NextCheck: retryAt.Add(time.Second)}
	if saved, err := store.SaveStatusCheck(t.Context(), claimed[0], result); err != nil || saved {
		t.Fatalf("late result replaced new claim: %v, %v", saved, err)
	}
	if saved, err := store.SaveStatusCheck(t.Context(), newer[0], result); err != nil || !saved {
		t.Fatalf("current claim could not complete: %v, %v", saved, err)
	}
}
