package main

import (
	"fmt"
	"os"
	"time"
)

func applyOptionalEnv(cfg Config) (Config, error) {
	var err error
	cfg, err = parseRedisDB(cfg)
	if err != nil {
		return Config{}, err
	}
	cfg, err = parseDurationEnvs(cfg)
	if err != nil {
		return Config{}, err
	}
	return parseMaxAttempts(cfg)
}

func parseRedisDB(cfg Config) (Config, error) {
	raw := os.Getenv("COACH_REDIS_DB")
	if raw == "" {
		return cfg, nil
	}
	var db int
	if _, err := fmt.Sscanf(raw, "%d", &db); err != nil {
		return Config{}, fmt.Errorf("coach-worker: invalid COACH_REDIS_DB %q: %w", raw, err)
	}
	cfg.RedisDB = db
	return cfg, nil
}

func parseDurationEnvs(cfg Config) (Config, error) {
	type field struct {
		env string
		set func(Config, time.Duration) Config
	}
	fields := []field{
		{"COACH_WORKER_HEARTBEAT_INTERVAL", func(c Config, d time.Duration) Config { c.HeartbeatInterval = d; return c }},
		{"COACH_WORKER_STALE_AFTER", func(c Config, d time.Duration) Config { c.StaleAfter = d; return c }},
		{"COACH_WORKER_RECONCILE_INTERVAL", func(c Config, d time.Duration) Config { c.ReconcileInterval = d; return c }},
		{"COACH_WORKER_QUEUED_AGE_THRESHOLD", func(c Config, d time.Duration) Config { c.QueuedAgeThreshold = d; return c }},
		{"COACH_WORKER_IDLE_POLL_INTERVAL", func(c Config, d time.Duration) Config { c.IdlePollInterval = d; return c }},
		{"COACH_REDIS_CLAIM_AFTER", func(c Config, d time.Duration) Config { c.RedisClaimAfter = d; return c }},
	}
	for _, f := range fields {
		d, ok, err := parseDurationEnv(f.env)
		if err != nil {
			return Config{}, err
		}
		if ok {
			cfg = f.set(cfg, d)
		}
	}
	return cfg, nil
}

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
