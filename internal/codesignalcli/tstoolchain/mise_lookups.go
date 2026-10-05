package tstoolchain

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// DetectGlobalMiseTypescriptVersion reports the globally configured
// TypeScript version. A non-zero exit means the key is unset, not that the
// probe failed.
var DetectGlobalMiseTypescriptVersion = func(ctx context.Context) (version string, found bool) {
	out, _, ok := runMiseProbe(ctx, "config", "get", "tools."+MiseNpmTypescriptTool, "-g")
	if !ok || out == "" {
		return "", false
	}
	return out, true
}

// LocateMiseTypescriptInstall reports where mise installed the requested
// version. `mise where` reports the tool's install root, not the npm package
// directory: the `typescript` package lives at
// <install-root>/node_modules/typescript.
var LocateMiseTypescriptInstall = func(ctx context.Context, version string) (string, bool) {
	toolRoot, exitErr, ok := runMiseProbe(ctx, "where", MiseNpmTypescriptTool+"@"+version)
	if !ok || exitErr != nil || toolRoot == "" {
		return "", false
	}
	pkgDir := filepath.Join(toolRoot, "node_modules", "typescript")
	if _, statErr := os.Stat(filepath.Join(pkgDir, "package.json")); statErr != nil {
		return "", false
	}
	return pkgDir, true
}

// ProbeMiseToolVersion runs the confined `mise --version` probe. It reports
// the raw trimmed output; extracting the leading calver token is
// project_ts_compiler_mise_version.go's concern, kept separate so this
// function stays a pure I/O probe like its siblings above. ok is false
// whenever the probe could not be confined, mise is absent from PATH, or the
// probe exited non-zero -- every one of those is "undetectable", never
// "unsupported".
var ProbeMiseToolVersion = func(ctx context.Context) (string, bool) {
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

// ProbeMiseGlobalConfigHazard scans every mise config file that resolves
// ambiently -- i.e. from runMiseProbe's own private, empty working
// directory, which by construction has no project-local mise.toml of its
// own -- for the same execution/redirection constructs HasMiseConfigHazard
// checks in a project's mise.toml. It fails closed: any inability to
// enumerate or read those files (mise config ls erroring, unparseable JSON,
// an unreadable listed file) is treated as a hazard rather than silently
// reported as safe.
var ProbeMiseGlobalConfigHazard = func(ctx context.Context) bool {
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
		if HasMiseConfigHazard(string(data)) {
			return true
		}
	}
	return false
}
