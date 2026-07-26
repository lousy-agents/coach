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

func parseJudgmentEnv(cfg Config) (Config, error) {
	d, ok, err := parseDurationEnv("COACH_JUDGMENT_MAX_WALL_TIME")
	if err != nil {
		return Config{}, err
	}
	if ok {
		if d < minJudgmentMaxWallTime {
			return Config{}, fmt.Errorf(
				"coach-worker: invalid COACH_JUDGMENT_MAX_WALL_TIME %q (must be >= %s)",
				os.Getenv("COACH_JUDGMENT_MAX_WALL_TIME"), minJudgmentMaxWallTime,
			)
		}
		cfg.JudgmentMaxWallTime = d
	}

	if raw := os.Getenv("COACH_MAX_HIDDEN_MUTATION_JUDGMENTS"); raw != "" {
		var n int
		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil {
			return Config{}, fmt.Errorf("coach-worker: invalid COACH_MAX_HIDDEN_MUTATION_JUDGMENTS %q (must be integer; 0=default 16; negative=unlimited)", raw)
		}
		cfg.MaxHiddenMutationJudgments = n
	}

	type intField struct {
		env string
		set func(Config, int) Config
	}
	packFields := []intField{
		{"COACH_MAX_FINDINGS_PER_JUDGMENT_PACK", func(c Config, n int) Config { c.MaxFindingsPerJudgmentPack = n; return c }},
		{"COACH_MAX_JUDGMENT_PROMPT_TOKENS", func(c Config, n int) Config { c.MaxJudgmentPromptTokens = n; return c }},
		{"COACH_JUDGMENT_FILE_AFFINITY_MIN_FINDINGS", func(c Config, n int) Config { c.JudgmentFileAffinityMinFindings = n; return c }},
		{"COACH_JUDGMENT_EVIDENCE_WINDOW_LINES", func(c Config, n int) Config { c.JudgmentEvidenceWindowLines = n; return c }},
	}
	for _, f := range packFields {
		raw := os.Getenv(f.env)
		if raw == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 1 {
			return Config{}, fmt.Errorf("coach-worker: invalid %s %q (must be integer >= 1)", f.env, raw)
		}
		cfg = f.set(cfg, n)
	}
	return cfg, nil
}

func parseBaselineBudgets(cfg Config) (Config, error) {
	// 0 = unlimited (handler contract); negative rejected; unset keeps defaults.
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
