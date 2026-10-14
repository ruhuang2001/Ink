package postgres

import (
	"context"
	"time"

	"github.com/ruhuang/ink/server/internal/platform/observability"
)

// SampleBacklog mirrors claim eligibility, excluding unexpired leases. Print
// confirmation/device waiting populations are informational, never overdue work.
func (s *Store) SampleBacklog(ctx context.Context, now time.Time) ([]observability.BacklogSample, error) {
	rows, err := s.db.Query(ctx, `
		select 'plugin_fetch', count(*), min(next_fetch_at) from plugin_bindings
		where enabled and status = 'connected' and next_fetch_at <= $1
		  and (fetch_lease_until is null or fetch_lease_until < $1)
		union all
		select 'schedule', count(*), min(next_run_at) from print_schedules
		where enabled and next_run_at <= $1 and (lease_until is null or lease_until < $1)
		union all
		select 'print_status', count(*), min(due_at) from (
		  select coalesce(j.next_status_check_at, j.updated_at) as due_at
		  from print_jobs j join printer_bindings b on b.id = j.printer_binding_id and b.user_id = j.user_id
		  where j.status = 'queued' and j.provider_print_content_id is not null
		    and (j.next_status_check_at is null or j.next_status_check_at <= $1)
		  union all
		  select updated_at + interval '3 minutes' from print_jobs
		  where status = 'queued' and provider_print_content_id is null
		    and updated_at <= $1::timestamptz - interval '3 minutes'
		) due
		union all
		select 'pending_confirmation', count(*), min(created_at) from print_jobs where status = 'pending'
		union all
		select 'submitted_print', count(*), min(created_at) from print_jobs
		where status = 'queued' and provider_print_content_id is not null
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var samples []observability.BacklogSample
	for rows.Next() {
		var sample observability.BacklogSample
		var oldest *time.Time
		if err := rows.Scan(&sample.Queue, &sample.Count, &oldest); err != nil {
			return nil, err
		}
		if oldest != nil {
			sample.Oldest = *oldest
		}
		samples = append(samples, sample)
	}
	return samples, rows.Err()
}
