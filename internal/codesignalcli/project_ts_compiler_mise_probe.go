package codesignalcli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	miseProbeTimeout   = 10 * time.Second
	maxMiseProbeOutput = 4 << 10
)

// runMiseProbe runs one read-only `mise` subcommand -- never `mise install`
// or any other mutating subcommand. ok is false unless the probe ran fully
// confined, so a probe that cannot be confined does not run at all. A
// non-zero exit is reported in exitErr rather than as a failed probe.
func runMiseProbe(ctx context.Context, args ...string) (out string, exitErr error, ok bool) {
	if _, lookErr := exec.LookPath("mise"); lookErr != nil {
		return "", nil, false
	}
	// The working directory is private per probe rather than a fixed shared
	// path: mise resolves configuration by walking up from it, so a
	// predictable path under TMPDIR would let another local user plant
	// configuration the probe then loads.
	probeDir, tempErr := os.MkdirTemp("", "coach-mise-probe-")
	if tempErr != nil {
		return "", nil, false
	}
	defer func() { _ = os.RemoveAll(probeDir) }()

	data, exitErr, probeErr := runBoundedSubprocessProbeAt(ctx, miseProbeTimeout, maxMiseProbeOutput, probeDir, miseProbeEnv(), "mise", args...)
	if probeErr != nil {
		return "", nil, false
	}
	return strings.TrimSpace(string(data)), exitErr, true
}

func miseProbeEnv() []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
	for _, key := range []string{"MISE_DATA_DIR", "MISE_CONFIG_DIR"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// detectGlobalMiseTypescriptVersion reports the globally configured
// TypeScript version. A non-zero exit means the key is unset, not that the
// probe failed.
var detectGlobalMiseTypescriptVersion = func(ctx context.Context) (version string, found bool) {
	out, _, ok := runMiseProbe(ctx, "config", "get", "tools."+miseNpmTypescriptTool, "-g")
	if !ok || out == "" {
		return "", false
	}
	return out, true
}

// locateMiseTypescriptInstall reports where mise installed the requested
// version. `mise where` reports the tool's install root, not the npm package
// directory: the `typescript` package lives at
// <install-root>/node_modules/typescript.
var locateMiseTypescriptInstall = func(ctx context.Context, version string) (string, bool) {
	toolRoot, exitErr, ok := runMiseProbe(ctx, "where", miseNpmTypescriptTool+"@"+version)
	if !ok || exitErr != nil || toolRoot == "" {
		return "", false
	}
	pkgDir := filepath.Join(toolRoot, "node_modules", "typescript")
	if _, statErr := os.Stat(filepath.Join(pkgDir, "package.json")); statErr != nil {
		return "", false
	}
	return pkgDir, true
}

// probeMiseToolVersion runs the confined `mise --version` probe. It reports
// the raw trimmed output; extracting the leading calver token is
// project_ts_compiler_mise_version.go's concern, kept separate so this
// function stays a pure I/O probe like its siblings above. ok is false
// whenever the probe could not be confined, mise is absent from PATH, or the
// probe exited non-zero -- every one of those is "undetectable", never
// "unsupported".
var probeMiseToolVersion = func(ctx context.Context) (string, bool) {
	out, exitErr, ok := runMiseProbe(ctx, "--version")
	if !ok || exitErr != nil || out == "" {
		return "", false
	}
	return out, true
}

// miseConfigListEntry mirrors the one field of `mise config ls -J` this
// package reads: the absolute path of each config file mise actually loaded.
type miseConfigListEntry struct {
	Path string `json:"path"`
}

// probeMiseGlobalConfigHazard scans every mise config file that resolves
// ambiently -- i.e. from runMiseProbe's own private, empty working
// directory, which by construction has no project-local mise.toml of its
// own -- for the same execution/redirection constructs hasMiseConfigHazard
// checks in a project's mise.toml (AC-9's repository-controlled framing,
// extended here for defense in depth per project_ts_compiler_mise_version.go's
// package comment). It fails closed: any inability to enumerate or read
// those files (mise config ls erroring, unparseable JSON, an unreadable
// listed file) is treated as a hazard rather than silently reported as safe.
var probeMiseGlobalConfigHazard = func(ctx context.Context) bool {
	out, exitErr, ok := runMiseProbe(ctx, "config", "ls", "-J")
	if !ok || exitErr != nil {
		return true
	}
	var entries []miseConfigListEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		return true
	}
	for _, entry := range entries {
		data, err := os.ReadFile(entry.Path)
		if err != nil {
			return true
		}
		if hasMiseConfigHazard(string(data)) {
			return true
		}
	}
	return false
}

// readMiseProjectConfigFile reads a project's mise.toml for hazard scanning.
// A missing or unreadable file is not itself a hazard signal here --
// evaluateMiseProjectOrigin already reports that distinctly (unconfigured or
// unreadable); this just has nothing to scan.
func readMiseProjectConfigFile(dir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "mise.toml"))
	if err != nil {
		return "", false
	}
	return string(data), true
}
