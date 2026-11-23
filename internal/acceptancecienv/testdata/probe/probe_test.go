package probe

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// scrubbedVariables are the variables acceptance-ci-env removes, because CI
// sets none of them; the probe reports which of them reached it.
var scrubbedVariables = []string{
	"GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", "GIT_CONFIG_PARAMETERS",
	"GIT_CONFIG_SYSTEM", "GIT_CONFIG_NOSYSTEM", "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL",
	"GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL",
}

// TestRecordTheConditions writes what this test process sees to the file
// COACH_CI_ENV_PROBE_OUT names, so the acceptance specs can check the
// conditions acceptance-ci-env ran the tests under.
func TestRecordTheConditions(t *testing.T) {
	out := os.Getenv("COACH_CI_ENV_PROBE_OUT")
	if out == "" {
		t.Fatal("COACH_CI_ENV_PROBE_OUT is not set")
	}
	home, homeEntries, homeErr := listed("HOME")
	tmp, tmpEntries, tmpErr := listed("TMPDIR")
	globalConfig, globalErr := exec.Command("git", "config", "--global", "--list").CombinedOutput()
	version, _ := exec.Command("git", "version").Output()
	_, goErr := exec.LookPath("go")
	scoped, _ := exec.Command("git", "config", "--list", "--show-scope").Output()
	var fromCommandLine, scopes []string
	for _, line := range strings.Split(string(scoped), "\n") {
		scope, _, _ := strings.Cut(line, "\t")
		if scope == "command" {
			fromCommandLine = append(fromCommandLine, line)
		}
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	var set []string
	for _, name := range scrubbedVariables {
		if _, ok := os.LookupEnv(name); ok {
			set = append(set, name)
		}
	}
	record := map[string]any{
		"uid":                 os.Getuid(),
		"home":                home,
		"home_entries":        homeEntries,
		"home_read_error":     homeErr,
		"tmpdir":              tmp,
		"tmpdir_entries":      tmpEntries,
		"tmpdir_read_error":   tmpErr,
		"global_config":       strings.TrimSpace(string(globalConfig)),
		"global_config_error": globalErr != nil,
		"git_version":         strings.TrimSpace(string(version)),
		"go_on_path":          goErr == nil,
		"config_from_env":     fromCommandLine,
		"config_scopes":       scopes,
		"scrubbed_set":        set,
		"js_installed":        readInstalledJS(),
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// listed reads the directory a variable names, and says so when it cannot,
// so that an unreadable directory never passes for an empty one.
func listed(variable string) (dir string, entries []string, readErr string) {
	dir = os.Getenv(variable)
	found, err := os.ReadDir(dir)
	if err != nil {
		return dir, nil, err.Error()
	}
	for _, entry := range found {
		entries = append(entries, entry.Name())
	}
	return dir, entries, ""
}
