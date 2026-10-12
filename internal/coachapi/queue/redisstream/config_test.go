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
			expectValidateRejects(t, baseValid(), tc.mutate, tc.name)
		})
	}
}

// expectValidateRejects applies mutate to an otherwise valid cfg and
// requires Validate to reject the result.
func expectValidateRejects(t *testing.T, cfg Config, mutate func(*Config), name string) {
	t.Helper()
	mutate(&cfg)
	if err := cfg.Validate(); err == nil {
		t.Fatalf("Validate() = nil, want an error for %s", name)
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
