package main

import (
	"fmt"
	"os"
	"time"
)

func applyOptionalEnv(cfg *Config) error {
	if err := parseRedisDB(cfg); err != nil {
		return err
	}
	if err := parseDurationEnvs(cfg); err != nil {
		return err
	}
	return parseMaxAttempts(cfg)
}

func parseRedisDB(cfg *Config) error {
	raw := os.Getenv("COACH_REDIS_DB")
	if raw == "" {
		return nil
	}
	var db int
	if _, err := fmt.Sscanf(raw, "%d", &db); err != nil {
		return fmt.Errorf("coach-worker: invalid COACH_REDIS_DB %q: %w", raw, err)
	}
	cfg.RedisDB = db
	return nil
}

func parseDurationEnvs(cfg *Config) error {
	pairs := []struct {
		env string
		dst *time.Duration
	}{
		{"COACH_WORKER_HEARTBEAT_INTERVAL", &cfg.HeartbeatInterval},
		{"COACH_WORKER_STALE_AFTER", &cfg.StaleAfter},
		{"COACH_WORKER_RECONCILE_INTERVAL", &cfg.ReconcileInterval},
		{"COACH_WORKER_QUEUED_AGE_THRESHOLD", &cfg.QueuedAgeThreshold},
		{"COACH_WORKER_IDLE_POLL_INTERVAL", &cfg.IdlePollInterval},
		{"COACH_REDIS_CLAIM_AFTER", &cfg.RedisClaimAfter},
	}
	for _, pair := range pairs {
		if err := parseDurationEnv(pair.env, pair.dst); err != nil {
			return err
		}
	}
	return nil
}

func parseDurationEnv(env string, dst *time.Duration) error {
	raw := os.Getenv(env)
	if raw == "" {
		return nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("coach-worker: invalid %s %q: %w", env, raw, err)
	}
	*dst = d
	return nil
}

func parseMaxAttempts(cfg *Config) error {
	raw := os.Getenv("COACH_WORKER_MAX_ATTEMPTS")
	if raw == "" {
		return nil
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 1 {
		return fmt.Errorf("coach-worker: invalid COACH_WORKER_MAX_ATTEMPTS %q (must be integer >= 1)", raw)
	}
	cfg.MaxAttempts = n
	return nil
}
