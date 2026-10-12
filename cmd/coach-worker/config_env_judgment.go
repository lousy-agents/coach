package main

import (
	"fmt"
	"os"
)

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
