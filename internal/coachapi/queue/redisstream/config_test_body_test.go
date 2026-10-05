package redisstream

import (
	"testing"
)

func body_configTest_validConfigPasses_18(t *testing.T, baseValid func() Config) {
	if err := baseValid().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func body_configTest_35(t *testing.T, baseValid func() Config, tc struct {
	name   string
	mutate func(*Config)
}) {
	cfg := baseValid()
	tc.mutate(&cfg)
	if err := cfg.Validate(); err == nil {
		t.Fatalf("Validate() = nil, want an error for %s", tc.name)
	}
}
