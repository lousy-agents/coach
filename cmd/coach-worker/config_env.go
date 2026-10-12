package main

import (
	"fmt"
	"os"
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
	cfg, err = parseMaxAttempts(cfg)
	if err != nil {
		return Config{}, err
	}
	cfg, err = parseBaselineBudgets(cfg)
	if err != nil {
		return Config{}, err
	}
	cfg, err = parseJudgmentEnv(cfg)
	if err != nil {
		return Config{}, err
	}
	return parseGitHubAppEnv(cfg)
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
