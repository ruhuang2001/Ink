package postgres

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/schedule"
	"github.com/ruhuang/ink/server/internal/workspace"
)

func TestRemovePrinterPreservesHistoryAndDisablesSchedules(t *testing.T) {
	ctx := t.Context()
	db := newIsolatedTestDB(t, "printer_remove")
	for version := 1; version <= 10; version++ {
		applyMigration(t, ctx, db, version)
	}
	fixture, err := os.ReadFile("../../../../testdata/printer_history.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, string(fixture))
	if err != nil {
		t.Fatal(err)
	}
	store := New(db)
	staleSchedule, err := store.FindByID(ctx, "remove-user", "remove-schedule")
	if err != nil || staleSchedule == nil {
		t.Fatalf("load original schedule: %+v, %v", staleSchedule, err)
	}
	if err := store.DeleteBinding(ctx, "other-user", "remove-device"); err != nil {
		t.Fatal(err)
	}
	binding, err := store.FindBindingByID(ctx, "remove-user", "remove-device")
	if err != nil || binding == nil || binding.Status != workspace.DeviceStatusConnected {
		t.Fatalf("another user removed the binding: %+v, %v", binding, err)
	}
	if err := store.DeleteBinding(ctx, "remove-user", "remove-device"); err != nil {
		t.Fatal(err)
	}
	binding, err = store.FindBindingByID(ctx, "remove-user", "remove-device")
	if err != nil || binding == nil || binding.Status != workspace.DeviceStatusOffline {
		t.Fatalf("binding was not preserved offline: %+v, %v", binding, err)
	}
	current, err := store.FindByID(ctx, "remove-user", "remove-schedule")
	if err != nil || current == nil || current.Enabled || current.LeaseUntil != nil {
		t.Fatalf("schedule was not preserved disabled: %+v, %v", current, err)
	}
	for _, jobID := range []string{"completed-job", "queued-job", "pending-job"} {
		job, err := store.FindJobByID(ctx, "remove-user", jobID)
		if err != nil || job == nil || job.Content == "" || job.PrinterBindingID != binding.ID {
			t.Fatalf("print content disappeared: %+v, %v", job, err)
		}
	}
	var deliveryJobID string
	if err := db.QueryRow(ctx, "select print_job_id from print_schedule_deliveries where id = 'remove-delivery'").Scan(&deliveryJobID); err != nil || deliveryJobID != "completed-job" {
		t.Fatalf("delivery history disappeared: %q, %v", deliveryJobID, err)
	}
	counts, _, err := store.GetJobCounts(ctx, "remove-user", time.Now().Add(-time.Hour))
	if err != nil || counts.Completed != 1 || counts.Queued != 1 || counts.Pending != 1 {
		t.Fatalf("history counts changed: %+v, %v", counts, err)
	}
	history, err := store.ListJobSummaries(ctx, "remove-user", printer.JobPageQuery{Status: "history", Limit: 20})
	if err != nil || len(history) != 1 || history[0].ID != "completed-job" {
		t.Fatalf("history is not readable: %+v, %v", history, err)
	}
	due, err := store.ClaimDueStatusJobs(ctx, time.Now().Add(time.Minute), time.Now().Add(2*time.Minute), 20)
	if err != nil || len(due) != 1 || due[0].ID != "queued-job" {
		t.Fatalf("accepted job lost status synchronization: %+v, %v", due, err)
	}
	if err := store.Save(ctx, *staleSchedule); !errors.Is(err, schedule.ErrConflict) {
		t.Fatal(err)
	}
	current, err = store.FindByID(ctx, "remove-user", "remove-schedule")
	if err != nil || current.Enabled {
		t.Fatalf("stale runner re-enabled removed device schedule: %+v, %v", current, err)
	}
	claimed, err := store.ClaimDue(ctx, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour), 20)
	if err != nil || len(claimed) != 0 {
		t.Fatalf("disabled schedule remained eligible: %+v, %v", claimed, err)
	}
	binding.Status = workspace.DeviceStatusConnected
	binding.UpdatedAt = time.Now().UTC()
	if err := store.SaveBinding(ctx, *binding); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, *staleSchedule); !errors.Is(err, schedule.ErrConflict) {
		t.Fatal(err)
	}
	current, err = store.FindByID(ctx, "remove-user", "remove-schedule")
	if err != nil || current.Enabled {
		t.Fatalf("rebind resumed schedule without approval: %+v, %v", current, err)
	}
	current.Enabled = true
	current.UpdatedAt = time.Now().UTC()
	if err := store.Save(ctx, *current); err != nil {
		t.Fatal(err)
	}
	current, err = store.FindByID(ctx, "remove-user", "remove-schedule")
	if err != nil || !current.Enabled {
		t.Fatalf("schedule could not resume after rebind: %+v, %v", current, err)
	}
}
