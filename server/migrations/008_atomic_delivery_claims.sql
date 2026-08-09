alter table print_schedule_deliveries
  add column if not exists lease_until timestamptz null;

alter table print_schedule_deliveries
  drop constraint if exists print_schedule_deliveries_status_check;

alter table print_schedule_deliveries
  add constraint print_schedule_deliveries_status_check
  check (status in ('reserved', 'printed', 'failed'));

create index if not exists print_schedule_deliveries_retryable_idx
  on print_schedule_deliveries (print_schedule_id, status, lease_until, updated_at asc);
