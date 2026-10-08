alter table workspace_snapshots
  add column if not exists revision bigint not null default 1 check (revision > 0);
