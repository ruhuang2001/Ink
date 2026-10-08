package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ruhuang/ink/server/internal/schedule"
	"github.com/ruhuang/ink/server/internal/workspace"
)

var _ schedule.Repository = (*Store)(nil)

func (s *Store) ListByUserID(ctx context.Context, userID string) ([]schedule.PrintSchedule, error) {
	rows, err := s.db.Query(ctx, `
		select id, user_id, plugin_installation_id, plugin_binding_id, title, frequency_type,
			timezone, hour, minute, weekdays, print_policy_json, device_id, enabled,
			next_run_at, last_run_at, lease_until, last_error, created_at, updated_at
		from print_schedules
		where user_id = $1
		order by updated_at desc, created_at desc
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []schedule.PrintSchedule{}
	for rows.Next() {
		current, err := scanPrintSchedule(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *current)
	}

	return result, rows.Err()
}

func (s *Store) FindByID(ctx context.Context, userID string, scheduleID string) (*schedule.PrintSchedule, error) {
	row := s.db.QueryRow(ctx, `
		select id, user_id, plugin_installation_id, plugin_binding_id, title, frequency_type,
			timezone, hour, minute, weekdays, print_policy_json, device_id, enabled,
			next_run_at, last_run_at, lease_until, last_error, created_at, updated_at
		from print_schedules
		where user_id = $1 and id = $2
	`, userID, scheduleID)
	return scanPrintSchedule(row)
}

func (s *Store) Save(ctx context.Context, current schedule.PrintSchedule) error {
	weekdaysJSON, err := json.Marshal(current.Weekdays)
	if err != nil {
		return err
	}
	printPolicyJSON, err := json.Marshal(current.PrintPolicy)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the binding before its schedules, matching device removal. A stale
	// runner must not re-enable a schedule after its device was removed.
	var deviceStatus workspace.DeviceStatus
	if err := tx.QueryRow(ctx, `
		select status from printer_bindings
		where id = $1 and user_id = $2 for update
	`, current.DeviceID, current.UserID).Scan(&deviceStatus); err != nil {
		return err
	}
	current.Enabled = current.Enabled && deviceStatus == workspace.DeviceStatusConnected

	_, err = tx.Exec(ctx, `
		insert into print_schedules (
			id, user_id, plugin_installation_id, plugin_binding_id, title, frequency_type,
			timezone, hour, minute, weekdays, print_policy_json, device_id, enabled,
			next_run_at, last_run_at, lease_until, last_error, created_at, updated_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		on conflict (id)
		do update set
			plugin_installation_id = excluded.plugin_installation_id,
			plugin_binding_id = excluded.plugin_binding_id,
			title = excluded.title,
			frequency_type = excluded.frequency_type,
			timezone = excluded.timezone,
			hour = excluded.hour,
			minute = excluded.minute,
			weekdays = excluded.weekdays,
			print_policy_json = excluded.print_policy_json,
			device_id = excluded.device_id,
			enabled = excluded.enabled,
			next_run_at = excluded.next_run_at,
			last_run_at = excluded.last_run_at,
			lease_until = excluded.lease_until,
			last_error = excluded.last_error,
			updated_at = excluded.updated_at
		where print_schedules.updated_at <= excluded.updated_at
	`,
		current.ID,
		current.UserID,
		current.PluginInstallationID,
		current.PluginBindingID,
		current.Title,
		current.FrequencyType,
		current.Timezone,
		current.Hour,
		current.Minute,
		weekdaysJSON,
		printPolicyJSON,
		current.DeviceID,
		current.Enabled,
		current.NextRunAt,
		current.LastRunAt,
		current.LeaseUntil,
		current.LastError,
		current.CreatedAt,
		current.UpdatedAt,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Delete(ctx context.Context, userID string, scheduleID string) error {
	_, err := s.db.Exec(ctx, `
		delete from print_schedules
		where user_id = $1 and id = $2
	`, userID, scheduleID)
	return err
}

func (s *Store) ClaimDue(ctx context.Context, now time.Time, leaseUntil time.Time, limit int) ([]schedule.PrintSchedule, error) {
	rows, err := s.db.Query(ctx, `
		with due as (
			select id
			from print_schedules
			where enabled = true
			  and next_run_at <= $1
			  and (lease_until is null or lease_until < $1)
			order by next_run_at asc
			limit $3
			for update skip locked
		)
		update print_schedules as schedules
		set lease_until = $2, updated_at = $1
		from due
		where schedules.id = due.id
		returning schedules.id, schedules.user_id, schedules.plugin_installation_id,
			schedules.plugin_binding_id, schedules.title, schedules.frequency_type,
			schedules.timezone, schedules.hour, schedules.minute, schedules.weekdays,
			schedules.print_policy_json, schedules.device_id, schedules.enabled,
			schedules.next_run_at, schedules.last_run_at, schedules.lease_until,
			schedules.last_error, schedules.created_at, schedules.updated_at
	`, now, leaseUntil, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []schedule.PrintSchedule{}
	for rows.Next() {
		current, err := scanPrintSchedule(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *current)
	}

	return result, rows.Err()
}

func scanPrintSchedule(row pgx.Row) (*schedule.PrintSchedule, error) {
	var current schedule.PrintSchedule
	var weekdaysJSON []byte
	var printPolicyJSON []byte
	var lastRunAt *time.Time
	var leaseUntil *time.Time
	var lastError *string
	if err := row.Scan(
		&current.ID,
		&current.UserID,
		&current.PluginInstallationID,
		&current.PluginBindingID,
		&current.Title,
		&current.FrequencyType,
		&current.Timezone,
		&current.Hour,
		&current.Minute,
		&weekdaysJSON,
		&printPolicyJSON,
		&current.DeviceID,
		&current.Enabled,
		&current.NextRunAt,
		&lastRunAt,
		&leaseUntil,
		&lastError,
		&current.CreatedAt,
		&current.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if len(weekdaysJSON) > 0 {
		if err := json.Unmarshal(weekdaysJSON, &current.Weekdays); err != nil {
			return nil, err
		}
	}
	if len(printPolicyJSON) > 0 {
		if err := json.Unmarshal(printPolicyJSON, &current.PrintPolicy); err != nil {
			return nil, err
		}
	}
	if current.Weekdays == nil {
		current.Weekdays = []int{}
	}
	if current.PrintPolicy.BatchSize <= 0 {
		current.PrintPolicy.BatchSize = 1
	}
	current.LastRunAt = lastRunAt
	current.LeaseUntil = leaseUntil
	current.LastError = lastError
	return &current, nil
}
