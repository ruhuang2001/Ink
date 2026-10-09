package config

import "testing"

func TestEnvPrefixListRejectsIPv4MappedCIDR(t *testing.T) {
	t.Setenv("TEST_PROXY_CIDRS", "::ffff:10.0.0.0/104")
	if _, err := envPrefixList("TEST_PROXY_CIDRS"); err == nil {
		t.Fatal("expected IPv4-mapped CIDR to be rejected")
	}
}

func TestTrustedProxyHeaderValue(t *testing.T) {
	for _, value := range []string{"", "forwarded", "x-forwarded-for", "Forwarded"} {
		if _, err := trustedProxyHeaderValue(value); err != nil {
			t.Fatalf("trustedProxyHeaderValue(%q): %v", value, err)
		}
	}
	if _, err := trustedProxyHeaderValue("both"); err == nil {
		t.Fatal("expected unsupported trusted proxy header to be rejected")
	}
}

func TestPrintStatusSyncConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("JWT_SECRET", "test-secret")
	for _, key := range []string{"ACCESS_TOKEN_TTL", "AI_ALLOW_INSECURE_PRIVATE_URL", "AI_CONFIG_ENCRYPTION_KEY", "AI_PROVIDER_TIMEOUT", "APP_NAME", "DATABASE_URL", "INBOX_JANITOR_INTERVAL", "INBOX_RETENTION", "JWT_SECRET", "LOGIN_RATE_LIMIT_MAX", "LOGIN_RATE_LIMIT_MAX_ENTRIES", "LOGIN_RATE_LIMIT_WINDOW", "MEMOBIRD_ACCESS_KEY", "MEMOBIRD_BASE_URL", "MEMOBIRD_TIMEOUT", "PLUGIN_ENV_ALLOWLIST", "PLUGIN_EXEC_TIMEOUT", "PLUGIN_FETCH_MAX_BLOCKS_PER_ITEM", "PLUGIN_FETCH_MAX_ITEMS", "PLUGIN_FETCH_MAX_TEXT_BYTES", "PLUGIN_FETCH_MAX_URL_BYTES", "PLUGIN_GIT_ALLOWED_HOSTS", "PLUGIN_INSTALL_TIMEOUT", "PLUGIN_OUTPUT_MAX_BYTES", "PLUGIN_ROOT", "PLUGIN_UPLOAD_MAX_BYTES", "PORT", "PRINT_STATUS_BATCH_SIZE", "PRINT_STATUS_POLL_INTERVAL", "PRINT_STATUS_SYNC_ENABLED", "PRINT_STATUS_TIMEOUT", "REFRESH_TOKEN_TTL", "SCHEDULER_POLL_INTERVAL", "TRUSTED_PROXY_CIDRS", "TRUSTED_PROXY_HEADER"} {
		t.Setenv(key, "")
	}
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("JWT_SECRET", "test-secret")
	cfg, err := Load()
	if err != nil || !cfg.PrintStatusSyncEnabled || cfg.PrintStatusPollInterval.String() != "2s" || cfg.PrintStatusBatchSize != 20 || cfg.PrintStatusTimeout.String() != "5s" {
		t.Fatalf("defaults: %+v, %v", cfg, err)
	}
	t.Setenv("PRINT_STATUS_SYNC_ENABLED", "False")
	cfg, err = Load()
	if err != nil || cfg.PrintStatusSyncEnabled {
		t.Fatalf("explicit disable: %+v, %v", cfg, err)
	}
	for _, test := range []struct{ key, value string }{
		{"PRINT_STATUS_POLL_INTERVAL", "0s"}, {"PRINT_STATUS_TIMEOUT", "-1s"}, {"PRINT_STATUS_BATCH_SIZE", "0"}, {"PRINT_STATUS_BATCH_SIZE", "101"},
	} {
		t.Run(test.key+test.value, func(t *testing.T) {
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected invalid worker setting to fail")
			}
		})
	}
}
