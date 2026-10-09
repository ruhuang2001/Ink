create index if not exists print_jobs_submission_recovery_idx
  on print_jobs (updated_at, id)
  where status = 'queued' and provider_print_content_id is null;
