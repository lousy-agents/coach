package redisstream

import (
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	baseValid := func() Config {
		return Config{
			Address:       "localhost:6379",
			Stream:        "coach-analysis",
			ConsumerGroup: "coach-workers",
			ClaimAfter:    time.Minute,
		}
	}

	t.Run("valid config passes", func(t *testing.T) {
		if err := baseValid().Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"missing Address", func(c *Config) { c.Address = "" }},
		{"missing Stream", func(c *Config) { c.Stream = "" }},
		{"missing ConsumerGroup", func(c *Config) { c.ConsumerGroup = "" }},
		{"zero ClaimAfter", func(c *Config) { c.ClaimAfter = 0 }},
		{"negative ClaimAfter", func(c *Config) { c.ClaimAfter = -time.Second }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseValid()
			tc.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want an error for %s", tc.name)
			}
		})
	}
}

func TestConfigSetDefaults(t *testing.T) {
	var cfg Config
	cfg.setDefaults()
	if cfg.DialTimeout != defaultDialTimeout {
		t.Fatalf("DialTimeout = %v, want default %v", cfg.DialTimeout, defaultDialTimeout)
	}

	cfg = Config{DialTimeout: 2 * time.Second}
	cfg.setDefaults()
	if cfg.DialTimeout != 2*time.Second {
		t.Fatalf("DialTimeout = %v, want unchanged 2s", cfg.DialTimeout)
	}
}

func TestPoisonStreamName(t *testing.T) {
	got := poisonStreamName("coach-analysis")
	want := "coach-analysis-poison"
	if got != want {
		t.Fatalf("poisonStreamName() = %q, want %q", got, want)
	}
}

// TestNewQueueValidatesConfigBeforeDialing proves NewQueue rejects an
// invalid Config without needing a reachable Redis instance -- it must
// fail on cfg.Validate(), not on a Ping timeout.
func TestNewQueueValidatesConfigBeforeDialing(t *testing.T) {
	_, err := NewQueue(Config{}, nil)
	if err == nil {
		t.Fatalf("NewQueue(zero Config) = nil error, want a validation error")
	}
}

// TestNewQueueFailsFastAgainstUnreachableRedis proves the outbound
// network policy (AGENTS.md "Outbound HTTP required policy", applied here
// to the Redis dial): NewQueue must not hang against an unreachable
// address, it must return within a small bound derived from
// Config.DialTimeout.
func TestNewQueueFailsFastAgainstUnreachableRedis(t *testing.T) {
	start := time.Now()
	_, err := NewQueue(Config{
		Address:       "127.0.0.1:1",
		Stream:        "coach-analysis",
		ConsumerGroup: "coach-workers",
		ClaimAfter:    time.Minute,
		DialTimeout:   200 * time.Millisecond,
	}, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("NewQueue against an unreachable address = nil error, want a connection error")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("NewQueue against an unreachable address took %v, want it to fail fast (bounded by DialTimeout)", elapsed)
	}
}
