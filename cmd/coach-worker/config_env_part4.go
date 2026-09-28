package main

import (
	"fmt"
	"os"
	"time"
)

func parseDurationEnv(env string) (time.Duration, bool, error) {
	raw := os.Getenv(env)
	if raw == "" {
		return 0, false, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, false, fmt.Errorf("coach-worker: invalid %s %q: %w", env, raw, err)
	}
	return d, true, nil
}
func parseMaxAttempts(cfg Config) (Config, error) {
	raw := os.Getenv("COACH_WORKER_MAX_ATTEMPTS")
	if raw == "" {
		return cfg, nil
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 1 {
		return Config{}, fmt.Errorf("coach-worker: invalid COACH_WORKER_MAX_ATTEMPTS %q (must be integer >= 1)", raw)
	}
	cfg.MaxAttempts = n
	return cfg, nil
}
