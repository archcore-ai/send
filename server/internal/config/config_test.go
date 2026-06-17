package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// A clean environment yields the public-instance defaults.
	for _, k := range []string{
		"SEND_LISTEN", "SEND_PUBLIC_URL", "SEND_DB_PATH", "SEND_BLOB_DIR",
		"SEND_DEFAULT_TTL", "SEND_MAX_TTL", "SEND_MAX_TOTAL_BYTES", "SEND_MAX_PART_BYTES",
		"SEND_MAX_PART_COUNT", "SEND_RATE_CREATE_PER_MIN", "SEND_TRUST_PROXY", "SEND_TEAM_TOKEN",
	} {
		t.Setenv(k, "")
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %s, want 1h", c.DefaultTTL)
	}
	if c.MaxTTL != 24*time.Hour {
		t.Errorf("MaxTTL = %s, want 24h", c.MaxTTL)
	}
	if c.MaxTotalBytes != 26214400 {
		t.Errorf("MaxTotalBytes = %d, want 26214400", c.MaxTotalBytes)
	}
	if c.MaxPartCount != 64 {
		t.Errorf("MaxPartCount = %d, want 64", c.MaxPartCount)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("SEND_DEFAULT_TTL", "30m")
	t.Setenv("SEND_MAX_TTL", "2h")
	t.Setenv("SEND_MAX_TOTAL_BYTES", "1048576")
	t.Setenv("SEND_MAX_PART_BYTES", "1048576")
	t.Setenv("SEND_TRUST_PROXY", "true")
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.DefaultTTL != 30*time.Minute {
		t.Errorf("DefaultTTL = %s, want 30m", c.DefaultTTL)
	}
	if !c.TrustProxy {
		t.Errorf("TrustProxy = false, want true")
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"ttl over ceiling", map[string]string{"SEND_MAX_TTL": "240h"}},
		{"default over max", map[string]string{"SEND_DEFAULT_TTL": "48h", "SEND_MAX_TTL": "24h"}},
		{"part over total", map[string]string{"SEND_MAX_PART_BYTES": "100", "SEND_MAX_TOTAL_BYTES": "50"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if _, err := Load(); err == nil {
				t.Fatalf("Load: expected error, got nil")
			}
		})
	}
}
