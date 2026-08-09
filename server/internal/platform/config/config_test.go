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
