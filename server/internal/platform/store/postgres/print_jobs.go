package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/ruhuang/ink/server/internal/printer"
)

func (s *Store) ListJobSummaries(ctx context.Context, userID string, query printer.JobPageQuery) ([]printer.JobSummaryRecord, error) {
	filter := ""
	switch query.Status {
	case "active":
		filter = " and status in ('pending', 'queued')"
	case "history":
		filter = " and status in ('completed', 'failed', 'cancelled')"
	}
	args := []any{userID, query.Limit}
	if query.Cursor != nil {
		filter += " and (created_at, id) < ($3, $4)"
		args = append(args, query.Cursor.CreatedAt, query.Cursor.ID)
	}
	rows, err := s.db.Query(ctx, `
		select id, title, source, printer_binding_id, status, created_at, updated_at
		from print_jobs where user_id = $1`+filter+`
		order by created_at desc, id desc limit $2
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]printer.JobSummaryRecord, 0, query.Limit)
	for rows.Next() {
		var job printer.JobSummaryRecord
		if err := rows.Scan(&job.ID, &job.Title, &job.Source, &job.DeviceID, &job.Status, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) ListJobStatuses(ctx context.Context, userID string, ids []string) ([]printer.JobStatusRecord, error) {
	rows, err := s.db.Query(ctx, `
		select id, printer_binding_id, status, updated_at
		from print_jobs where user_id = $1 and id = any($2)
		order by id
	`, userID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]printer.JobStatusRecord, 0, len(ids))
	for rows.Next() {
		var job printer.JobStatusRecord
		if err := rows.Scan(&job.ID, &job.DeviceID, &job.Status, &job.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) GetJobCounts(ctx context.Context, userID string, since time.Time) (printer.JobCounts, *string, error) {
	var counts printer.JobCounts
	var latestJobID *string
	err := s.db.QueryRow(ctx, `
		select count(*) filter (where status = 'pending'),
			count(*) filter (where status = 'queued'),
			count(*) filter (where status = 'completed'),
			count(*) filter (where status = 'failed'),
			count(*) filter (where status = 'cancelled'),
			count(*) filter (where status = 'completed' and updated_at >= $2),
			(select id from print_jobs where user_id = $1 order by created_at desc, id desc limit 1)
		from print_jobs where user_id = $1
	`, userID, since).Scan(&counts.Pending, &counts.Queued, &counts.Completed, &counts.Failed, &counts.Cancelled, &counts.TodayCompleted, &latestJobID)
	return counts, latestJobID, err
}

func (s *Store) ListDueStatusJobs(ctx context.Context, now time.Time, limit int) ([]printer.StatusSyncJob, error) {
	rows, err := s.db.Query(ctx, `
		select j.id, j.user_id, j.provider_print_content_id, j.updated_at,
			j.next_status_check_at, j.status_check_attempts,
			b.id, b.device_identifier, b.provider_user_id
		from print_jobs j join printer_bindings b on b.id = j.printer_binding_id and b.user_id = j.user_id
		where j.status = 'queued' and j.provider_print_content_id is not null and j.next_status_check_at <= $1
		order by j.next_status_check_at, j.id limit $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]printer.StatusSyncJob, 0, limit)
	for rows.Next() {
		var job printer.StatusSyncJob
		if err := rows.Scan(&job.ID, &job.UserID, &job.ProviderPrintID, &job.UpdatedAt,
			&job.NextStatusCheckAt, &job.Attempts, &job.Binding.ID, &job.Binding.DeviceIdentifier, &job.Binding.ProviderUserID); err != nil {
			return nil, err
		}
		job.Binding.UserID = job.UserID
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) SaveStatusCheck(ctx context.Context, job printer.StatusSyncJob, result printer.StatusCheckResult) (bool, error) {
	if result.NextCheck.IsZero() {
		return false, fmt.Errorf("next print status check is required")
	}
	tag, err := s.db.Exec(ctx, `
		update print_jobs set
			status = case when $7 then 'completed' else status end,
			updated_at = case when $7 then $8 else updated_at end,
			next_status_check_at = case when $7 then null::timestamptz else $9::timestamptz end,
			status_check_attempts = $10
		where id = $1 and user_id = $2 and status = 'queued'
			and provider_print_content_id = $3 and updated_at = $4
			and next_status_check_at = $5 and printer_binding_id = $6
	`, job.ID, job.UserID, job.ProviderPrintID, job.UpdatedAt, job.NextStatusCheckAt, job.Binding.ID,
		result.Completed, result.CheckedAt, result.NextCheck, result.Attempts)
	return tag.RowsAffected() == 1, err
}
