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
	for _, key := range []string{"PRINT_STATUS_SYNC_ENABLED", "PRINT_STATUS_POLL_INTERVAL", "PRINT_STATUS_BATCH_SIZE", "PRINT_STATUS_TIMEOUT"} {
		t.Setenv(key, "")
	}
	cfg, err := Load()
	if err != nil || !cfg.PrintStatusSyncEnabled || cfg.PrintStatusPollInterval.String() != "2s" || cfg.PrintStatusBatchSize != 20 || cfg.PrintStatusTimeout.String() != "5s" {
		t.Fatalf("defaults: %+v, %v", cfg, err)
	}
	t.Setenv("PRINT_STATUS_SYNC_ENABLED", "false")
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
