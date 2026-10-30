-- Memory system settings table
create table if not exists memory_settings (
  setting_key text primary key check (setting_key = 'system'),
  enabled boolean not null default false,
  zep_base_url text not null default '',
  zep_api_key_ciphertext bytea,
  zep_api_key_nonce bytea,
  updated_by text null references users (id) on delete set null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

-- Add comment for clarity
comment on table memory_settings is 'System-wide memory service configuration. Only one row with setting_key=system.';
comment on column memory_settings.enabled is 'Whether memory system is enabled. False disables all memory features.';
comment on column memory_settings.zep_base_url is 'Zep server base URL, e.g. http://zep:8000. Empty when disabled.';
