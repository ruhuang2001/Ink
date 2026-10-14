package postgres

import (
	"os"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/platform/observability"
)

func TestBacklogSeparatesDueWorkFromLeasesAndConfirmation(t *testing.T) {
	db := workspaceTestDatabase(t)
	applyMigration(t, t.Context(), db, 11)
	fixture, err := os.ReadFile("../../../../testdata/printer_history.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(t.Context(), string(fixture)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	old := now.Add(-10 * time.Minute)
	if _, err := db.Exec(t.Context(), `
		with bindings as (update plugin_bindings set next_fetch_at=$1 returning id),
		schedules as (update print_schedules set next_run_at=$1, lease_until=$2 returning id)
		update print_jobs set next_status_check_at=$1 where id='queued-job'
	`, old, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	store := New(db)
	samples, err := store.SampleBacklog(t.Context(), now)
	if err != nil {
		t.Fatal(err)
	}
	queues := make(map[string]observability.BacklogSample)
	for _, sample := range samples {
		queues[sample.Queue] = sample
	}
	for queue, count := range map[string]int64{"plugin_fetch": 1, "schedule": 0, "print_status": 1, "pending_confirmation": 1, "submitted_print": 1} {
		if queues[queue].Count != count {
			t.Fatalf("%s: got %+v, want count %d", queue, queues[queue], count)
		}
	}
	if !queues["print_status"].Oldest.Equal(old) || !queues["schedule"].Oldest.IsZero() {
		t.Fatalf("wrong due ages: %+v", queues)
	}
	// Future polling is device waiting, not due work. A provider-less submission
	// becomes actionable only at the same three-minute recovery boundary.
	if _, err := db.Exec(t.Context(), `
		with bindings as (update plugin_bindings set fetch_lease_until=$2 returning id),
		schedules as (update print_schedules set lease_until=$1 returning id),
		jobs as (update print_jobs set next_status_check_at=$2 where id='queued-job' returning id)
		insert into print_jobs(id,user_id,printer_binding_id,title,source,content,status,created_at,updated_at)
		values ('abandoned','remove-user','remove-device','Test','Manual','text','queued',$1,$1);
	`, now.Add(-3*time.Minute), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	samples, err = store.SampleBacklog(t.Context(), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range samples {
		queues[sample.Queue] = sample
	}
	if queues["plugin_fetch"].Count != 0 || queues["schedule"].Count != 1 || queues["print_status"].Count != 1 || !queues["print_status"].Oldest.Equal(now) || queues["submitted_print"].Count != 1 {
		t.Fatalf("lease or provider acceptance/recovery boundary mixed: %+v", queues)
	}
}
