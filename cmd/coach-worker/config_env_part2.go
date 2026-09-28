package main

import (
	"fmt"
	"os"
	"time"
)

func parseGitHubAppEnv(cfg Config) (Config, error) {
	if raw := os.Getenv("COACH_GITHUB_APP_ID"); raw != "" {
		var id int64
		if _, err := fmt.Sscanf(raw, "%d", &id); err != nil || id < 1 {
			return Config{}, fmt.Errorf("coach-worker: invalid COACH_GITHUB_APP_ID %q", raw)
		}
		cfg.GitHubAppID = id
	}
	if raw := os.Getenv("COACH_GITHUB_INSTALLATION_ID"); raw != "" {
		var id int64
		if _, err := fmt.Sscanf(raw, "%d", &id); err != nil || id < 1 {
			return Config{}, fmt.Errorf("coach-worker: invalid COACH_GITHUB_INSTALLATION_ID %q", raw)
		}
		cfg.GitHubInstallationID = id
	}
	if pem := os.Getenv("COACH_GITHUB_APP_PRIVATE_KEY"); pem != "" {
		cfg.GitHubPrivateKey = []byte(pem)
	} else if path := os.Getenv("COACH_GITHUB_APP_PRIVATE_KEY_PATH"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("coach-worker: reading COACH_GITHUB_APP_PRIVATE_KEY_PATH: %w", err)
		}
		cfg.GitHubPrivateKey = b
	}
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
