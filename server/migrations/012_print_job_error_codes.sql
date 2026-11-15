-- Failure messages moved from user-facing Chinese prose to stable codes that the
-- web client localizes. Rows written before this migration still hold the old
-- strings, and the API only serves values it recognizes, so without this they
-- would silently stop showing a recovery instruction.
update print_jobs
set error_message = 'print_submission_unknown'
where error_message = '提交结果未知，请先检查设备是否已打印，再手动创建新任务。';

-- Any other non-null message is an internal provider error. The API withholds
-- those deliberately, so clearing them loses nothing the user could see.
update print_jobs
set error_message = null
where error_message is not null
  and error_message <> 'print_submission_unknown';
