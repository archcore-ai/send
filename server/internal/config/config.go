// Package config loads the Send server configuration from environment variables,
// applies defaults, and validates the result. All knobs are SEND_*-prefixed and
// documented in deploy/.env.example.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// hardMaxTTL is the absolute ceiling for SEND_MAX_TTL (size-limits.rule: 7 days).
// An operator may configure a shorter max but never a longer one.
const hardMaxTTL = 7 * 24 * time.Hour

// Config is the validated server configuration.
type Config struct {
	Listen     string // SEND_LISTEN, host:port (":0" binds an ephemeral port and prints the URL)
	PublicURL  string // SEND_PUBLIC_URL; empty => derive base from the request Host
	DBPath     string // SEND_DB_PATH
	BlobDir    string // SEND_BLOB_DIR
	RequestLog string // SEND_REQUEST_LOG; empty => stderr

	DefaultTTL time.Duration // SEND_DEFAULT_TTL
	MaxTTL     time.Duration // SEND_MAX_TTL (<= 7d)
	GrantTTL   time.Duration // redeem grant window (fixed 10m in v0)
	GCInterval time.Duration // SEND_GC_INTERVAL

	MaxTotalBytes int64 // SEND_MAX_TOTAL_BYTES
	MaxPartBytes  int64 // SEND_MAX_PART_BYTES
	MaxPartCount  int   // SEND_MAX_PART_COUNT

	RateCreatePerMin int           // SEND_RATE_CREATE_PER_MIN (0 disables)
	RateUploadPerMin int           // SEND_RATE_UPLOAD_PER_MIN (0 disables)
	RateWindow       time.Duration // SEND_RATE_WINDOW
	TrustProxy       bool          // SEND_TRUST_PROXY (key rate-limit on X-Forwarded-For)

	TeamToken string // SEND_TEAM_TOKEN; empty => anonymous public mode
}

// Load reads the environment, applies defaults, and validates.
func Load() (Config, error) {
	c := Config{
		Listen:           env("SEND_LISTEN", ":8080"),
		PublicURL:        env("SEND_PUBLIC_URL", ""),
		DBPath:           env("SEND_DB_PATH", "data/sends.db"),
		BlobDir:          env("SEND_BLOB_DIR", "data/blobs"),
		RequestLog:       env("SEND_REQUEST_LOG", ""),
		DefaultTTL:       envDuration("SEND_DEFAULT_TTL", time.Hour),
		MaxTTL:           envDuration("SEND_MAX_TTL", 24*time.Hour),
		GrantTTL:         envDuration("SEND_GRANT_TTL", 10*time.Minute),
		GCInterval:       envDuration("SEND_GC_INTERVAL", 2*time.Minute),
		MaxTotalBytes:    envInt64("SEND_MAX_TOTAL_BYTES", 26214400), // 25 MiB
		MaxPartBytes:     envInt64("SEND_MAX_PART_BYTES", 26214400),
		MaxPartCount:     envInt("SEND_MAX_PART_COUNT", 64),
		RateCreatePerMin: envInt("SEND_RATE_CREATE_PER_MIN", 30),
		RateUploadPerMin: envInt("SEND_RATE_UPLOAD_PER_MIN", 240),
		RateWindow:       envDuration("SEND_RATE_WINDOW", time.Minute),
		TrustProxy:       envBool("SEND_TRUST_PROXY", false),
		TeamToken:        env("SEND_TEAM_TOKEN", ""),
	}
	if err := c.validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) validate() error {
	switch {
	case c.Listen == "":
		return fmt.Errorf("SEND_LISTEN must not be empty")
	case c.DBPath == "":
		return fmt.Errorf("SEND_DB_PATH must not be empty")
	case c.BlobDir == "":
		return fmt.Errorf("SEND_BLOB_DIR must not be empty")
	case c.DefaultTTL <= 0 || c.MaxTTL <= 0 || c.GrantTTL <= 0 || c.GCInterval <= 0:
		return fmt.Errorf("TTL/interval values must be positive")
	case c.MaxTTL > hardMaxTTL:
		return fmt.Errorf("SEND_MAX_TTL %s exceeds the 7d ceiling", c.MaxTTL)
	case c.DefaultTTL > c.MaxTTL:
		return fmt.Errorf("SEND_DEFAULT_TTL %s exceeds SEND_MAX_TTL %s", c.DefaultTTL, c.MaxTTL)
	case c.MaxTotalBytes <= 0 || c.MaxPartBytes <= 0:
		return fmt.Errorf("byte caps must be positive")
	case c.MaxPartBytes > c.MaxTotalBytes:
		return fmt.Errorf("SEND_MAX_PART_BYTES %d exceeds SEND_MAX_TOTAL_BYTES %d", c.MaxPartBytes, c.MaxTotalBytes)
	case c.MaxPartCount <= 0:
		return fmt.Errorf("SEND_MAX_PART_COUNT must be positive")
	case c.RateWindow <= 0:
		return fmt.Errorf("SEND_RATE_WINDOW must be positive")
	}
	return nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func envInt64(key string, def int64) int64 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func envInt(key string, def int) int {
	return int(envInt64(key, int64(def)))
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
