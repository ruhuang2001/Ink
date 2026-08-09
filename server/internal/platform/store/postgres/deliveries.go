package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ruhuang/ink/server/internal/dispatch"
	"github.com/ruhuang/ink/server/internal/inbox"
	"github.com/ruhuang/ink/server/internal/plugins"
)

var _ dispatch.Repository = (*Store)(nil)

const deliveryColumns = `id, print_schedule_id, plugin_item_id, status, attempt_count, last_error,
				print_job_id, delivered_at, lease_until, created_at, updated_at`

const listRetryableByScheduleQuery = `
	select
		items.id,
		items.user_id,
		items.plugin_installation_id,
		items.plugin_binding_id,
		items.device_id,
		items.external_id,
		items.title,
		items.source_label,
		items.published_at,
		items.blocks_json,
		items.status,
		items.attempt_count,
		items.last_error,
		items.print_job_id,
		items.fetched_at,
		items.created_at,
		items.updated_at,
		deliveries.id,
		deliveries.print_schedule_id,
		deliveries.plugin_item_id,
		deliveries.status,
		deliveries.attempt_count,
		deliveries.last_error,
		deliveries.print_job_id,
		deliveries.delivered_at,
		deliveries.lease_until,
		deliveries.created_at,
		deliveries.updated_at
	from print_schedule_deliveries deliveries
	join plugin_items items on items.id = deliveries.plugin_item_id
	where deliveries.print_schedule_id = $1
	  and ((deliveries.status = $2 and deliveries.attempt_count < $3)
	    or (deliveries.status = $4 and deliveries.lease_until <= now()))
	order by items.created_at asc, deliveries.updated_at asc
	limit $5
`

const listUndeliveredByScheduleQuery = `
	select
		items.id,
		items.user_id,
		items.plugin_installation_id,
		items.plugin_binding_id,
		items.device_id,
		items.external_id,
		items.title,
		items.source_label,
		items.published_at,
		items.blocks_json,
		items.status,
		items.attempt_count,
		items.last_error,
		items.print_job_id,
		items.fetched_at,
		items.created_at,
		items.updated_at
	from plugin_items items
	where items.plugin_binding_id = $1
	  and items.status = $2
	  and not exists (
		select 1
		from print_schedule_deliveries deliveries
		where deliveries.print_schedule_id = $3
		  and deliveries.plugin_item_id = items.id
	  )
	order by items.created_at asc
	limit $4
`

type scannedDeliveryRow struct {
	item           inbox.Item
	deviceID       *string
	publishedAt    *time.Time
	blocksJSON     []byte
	itemStatus     string
	itemLastError  *string
	itemPrintJobID *string
	delivery       dispatch.Delivery
	deliveryStatus string
}

func (s *Store) ListRetryableBySchedule(ctx context.Context, scheduleID string, limit int) ([]dispatch.DeliveryItem, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		ctx,
		listRetryableByScheduleQuery,
		scheduleID,
		string(dispatch.DeliveryStatusFailed),
		dispatch.MaxDeliveryAttempts,
		string(dispatch.DeliveryStatusReserved),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []dispatch.DeliveryItem{}
	for rows.Next() {
		item, delivery, err := scanDeliveryItem(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, dispatch.DeliveryItem{
			Item:     *item,
			Delivery: *delivery,
		})
	}
	return result, rows.Err()
}

func (s *Store) ClaimDelivery(ctx context.Context, delivery dispatch.Delivery, leaseUntil time.Time) (dispatch.Delivery, bool, error) {
	claimAt := leaseUntil.Add(-dispatch.ReservationLease)
	row := s.db.QueryRow(ctx, `
		insert into print_schedule_deliveries (
			id, print_schedule_id, plugin_item_id, status, attempt_count, last_error,
			print_job_id, delivered_at, lease_until, created_at, updated_at
		) values ($1, $2, $3, $4, 1, null, null, null, $5, $6, $7)
		on conflict (print_schedule_id, plugin_item_id) do update set
			status = excluded.status,
			attempt_count = print_schedule_deliveries.attempt_count + 1,
			last_error = null,
			print_job_id = null,
			delivered_at = null,
			lease_until = excluded.lease_until,
			updated_at = excluded.updated_at
		where (print_schedule_deliveries.status = $8 and print_schedule_deliveries.attempt_count < $9)
		   or (print_schedule_deliveries.status = $10 and print_schedule_deliveries.lease_until <= $7)
		returning `+deliveryColumns,
		delivery.ID, delivery.PrintScheduleID, delivery.PluginItemID,
		string(dispatch.DeliveryStatusReserved), leaseUntil, delivery.CreatedAt, claimAt,
		string(dispatch.DeliveryStatusFailed), dispatch.MaxDeliveryAttempts,
		string(dispatch.DeliveryStatusReserved),
	)
	claimed, err := scanDelivery(row)
	if err == pgx.ErrNoRows {
		return dispatch.Delivery{}, false, nil
	}
	if err != nil {
		return dispatch.Delivery{}, false, err
	}
	return claimed, true, nil
}

func (s *Store) ListUndeliveredBySchedule(ctx context.Context, scheduleID string, bindingID string, limit int) ([]inbox.Item, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		ctx,
		listUndeliveredByScheduleQuery,
		bindingID,
		string(inbox.StatusPending),
		scheduleID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []inbox.Item{}
	for rows.Next() {
		current, err := scanInboxItem(rows)
		if err != nil {
			return nil, err
		}
		if current != nil {
			result = append(result, *current)
		}
	}
	return result, rows.Err()
}

func (s *Store) SaveDelivery(ctx context.Context, delivery dispatch.Delivery) error {
	_, err := s.db.Exec(ctx, `
		insert into print_schedule_deliveries (
			id, print_schedule_id, plugin_item_id, status, attempt_count, last_error,
			print_job_id, delivered_at, lease_until, created_at, updated_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		on conflict (print_schedule_id, plugin_item_id)
		do update set
			status = excluded.status,
			attempt_count = excluded.attempt_count,
			last_error = excluded.last_error,
			print_job_id = excluded.print_job_id,
			delivered_at = excluded.delivered_at,
			lease_until = excluded.lease_until,
			updated_at = excluded.updated_at
	`,
		delivery.ID,
		delivery.PrintScheduleID,
		delivery.PluginItemID,
		string(delivery.Status),
		delivery.AttemptCount,
		delivery.LastError,
		delivery.PrintJobID,
		delivery.DeliveredAt,
		delivery.LeaseUntil,
		delivery.CreatedAt,
		delivery.UpdatedAt,
	)
	return err
}

func (s *Store) CountPrintedInLast24h(ctx context.Context, bindingID string, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, `
		select count(*)
		from print_schedule_deliveries deliveries
		join print_schedules schedules on schedules.id = deliveries.print_schedule_id
		where schedules.plugin_binding_id = $1
		  and deliveries.status = $2
		  and deliveries.updated_at >= $3
	`, bindingID, string(dispatch.DeliveryStatusPrinted), since).Scan(&count)
	return count, err
}

func scanDeliveryItem(row pgx.Row) (*inbox.Item, *dispatch.Delivery, error) {
	scanned, err := scanDeliveryRow(row)
	if err != nil {
		return nil, nil, err
	}
	return buildDeliveryItem(scanned)
}

func scanDeliveryRow(row pgx.Row) (scannedDeliveryRow, error) {
	var scanned scannedDeliveryRow
	err := row.Scan(
		&scanned.item.ID,
		&scanned.item.UserID,
		&scanned.item.PluginInstallationID,
		&scanned.item.PluginBindingID,
		&scanned.deviceID,
		&scanned.item.ExternalID,
		&scanned.item.Title,
		&scanned.item.SourceLabel,
		&scanned.publishedAt,
		&scanned.blocksJSON,
		&scanned.itemStatus,
		&scanned.item.AttemptCount,
		&scanned.itemLastError,
		&scanned.itemPrintJobID,
		&scanned.item.FetchedAt,
		&scanned.item.CreatedAt,
		&scanned.item.UpdatedAt,
		&scanned.delivery.ID,
		&scanned.delivery.PrintScheduleID,
		&scanned.delivery.PluginItemID,
		&scanned.deliveryStatus,
		&scanned.delivery.AttemptCount,
		&scanned.delivery.LastError,
		&scanned.delivery.PrintJobID,
		&scanned.delivery.DeliveredAt,
		&scanned.delivery.LeaseUntil,
		&scanned.delivery.CreatedAt,
		&scanned.delivery.UpdatedAt,
	)
	return scanned, err
}

func scanDelivery(row pgx.Row) (dispatch.Delivery, error) {
	var delivery dispatch.Delivery
	var status string
	err := row.Scan(&delivery.ID, &delivery.PrintScheduleID, &delivery.PluginItemID, &status,
		&delivery.AttemptCount, &delivery.LastError, &delivery.PrintJobID, &delivery.DeliveredAt,
		&delivery.LeaseUntil, &delivery.CreatedAt, &delivery.UpdatedAt)
	delivery.Status = dispatch.DeliveryStatus(status)
	return delivery, err
}

func buildDeliveryItem(scanned scannedDeliveryRow) (*inbox.Item, *dispatch.Delivery, error) {
	if len(scanned.blocksJSON) > 0 {
		if err := json.Unmarshal(scanned.blocksJSON, &scanned.item.Blocks); err != nil {
			return nil, nil, err
		}
	}
	scanned.item.DeviceID = scanned.deviceID
	scanned.item.PublishedAt = scanned.publishedAt
	scanned.item.Status = inbox.ItemStatus(scanned.itemStatus)
	scanned.item.LastError = scanned.itemLastError
	scanned.item.PrintJobID = scanned.itemPrintJobID
	if scanned.item.Blocks == nil {
		scanned.item.Blocks = []plugins.ContentBlock{}
	}
	scanned.delivery.Status = dispatch.DeliveryStatus(scanned.deliveryStatus)
	return &scanned.item, &scanned.delivery, nil
}
