alter table print_jobs
  add column if not exists next_status_check_at timestamptz null,
  add column if not exists status_check_attempts integer not null default 0;

update print_jobs
set next_status_check_at = now()
where status = 'queued' and provider_print_content_id is not null and next_status_check_at is null;

create index if not exists print_jobs_status_due_idx
  on print_jobs (next_status_check_at, id)
  where status = 'queued' and provider_print_content_id is not null;

create index if not exists print_jobs_user_created_idx
  on print_jobs (user_id, created_at desc, id desc);

create index if not exists print_jobs_user_active_created_idx
  on print_jobs (user_id, created_at desc, id desc)
  where status in ('pending', 'queued');
