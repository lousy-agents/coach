package probe

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestRecordTheConditions writes what this test process sees to the file
// COACH_CI_ENV_PROBE_OUT names, so the acceptance specs can check the
// conditions acceptance-ci-env ran the tests under.
func TestRecordTheConditions(t *testing.T) {
	out := os.Getenv("COACH_CI_ENV_PROBE_OUT")
	if out == "" {
		t.Fatal("COACH_CI_ENV_PROBE_OUT is not set")
	}
	home := os.Getenv("HOME")
	entries, _ := os.ReadDir(home)
	var homeEntries []string
	for _, entry := range entries {
		homeEntries = append(homeEntries, entry.Name())
	}
	globalConfig, globalErr := exec.Command("git", "config", "--global", "--list").CombinedOutput()
	version, _ := exec.Command("git", "version").Output()
	_, goErr := exec.LookPath("go")
	listed, _ := exec.Command("git", "config", "--list", "--show-origin").Output()
	var fromEnvironment []string
	for _, line := range strings.Split(string(listed), "\n") {
		if strings.HasPrefix(line, "command line:") {
			fromEnvironment = append(fromEnvironment, line)
		}
	}
	var gitIdentity []string
	for _, name := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		if _, set := os.LookupEnv(name); set {
			gitIdentity = append(gitIdentity, name)
		}
	}
	record := map[string]any{
		"uid":                 os.Getuid(),
		"home":                home,
		"home_entries":        homeEntries,
		"global_config":       strings.TrimSpace(string(globalConfig)),
		"global_config_error": globalErr != nil,
		"git_version":         strings.TrimSpace(string(version)),
		"go_on_path":          goErr == nil,
		"config_from_env":     fromEnvironment,
		"identity_from_env":   gitIdentity,
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
