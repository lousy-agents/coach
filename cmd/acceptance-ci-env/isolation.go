package main

import (
	"os"
	"path/filepath"
)

// isolatedGitAndHome makes the empty HOME, the empty user-level git config and
// the temporary directory the tests run with, all inside runDir.
func isolatedGitAndHome(runDir string) (map[string]string, error) {
	dirs := map[string]string{"HOME": "home", "XDG_CONFIG_HOME": "xdg", "TMPDIR": "tmp"}
	env := map[string]string{}
	for name, dir := range dirs {
		path := filepath.Join(runDir, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, err
		}
		env[name] = path
	}
	emptyConfig := filepath.Join(runDir, "gitconfig")
	if err := os.WriteFile(emptyConfig, nil, 0o644); err != nil {
		return nil, err
	}
	env["GIT_CONFIG_GLOBAL"] = emptyConfig
	return env, nil
}
