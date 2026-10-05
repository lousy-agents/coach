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
func parseBaselineBudgets(cfg Config) (Config, error) {

	if raw := os.Getenv("COACH_BASELINE_MAX_FILES"); raw != "" {
		var n int
		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 0 {
			return Config{}, fmt.Errorf("coach-worker: invalid COACH_BASELINE_MAX_FILES %q (must be integer >= 0; 0=unlimited)", raw)
		}
		cfg.BaselineMaxFiles = n
	}
	if raw := os.Getenv("COACH_BASELINE_MAX_TOTAL_BYTES"); raw != "" {
		var n int64
		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 0 {
			return Config{}, fmt.Errorf("coach-worker: invalid COACH_BASELINE_MAX_TOTAL_BYTES %q (must be integer >= 0; 0=unlimited)", raw)
		}
		cfg.BaselineMaxTotalBytes = n
	}
	return cfg, nil
}
