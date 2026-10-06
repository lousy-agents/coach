package main

import (
	"fmt"
	"os"
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
