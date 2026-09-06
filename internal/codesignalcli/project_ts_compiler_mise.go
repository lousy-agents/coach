package codesignalcli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const miseToolsSectionHeader = "[tools]"

// resolveMiseProjectCompiler is compiler-resolution origin 2: project mise
// configuration. Detection is a narrow, read-only scan for the
// "npm:typescript" key inside dir/mise.toml's [tools] table -- deliberately
// not a general TOML or mise-config parse -- so this never interprets
// [hooks], auto-run [tasks], _.source env directives, or any other hazard
// the frozen design calls out.
func resolveMiseProjectCompiler(dir string) compilerOriginOutcome {
	data, err := os.ReadFile(filepath.Join(dir, "mise.toml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return compilerOriginOutcome{state: compilerOutcomeEmpty}
		}
		return compilerOriginOutcome{state: compilerOutcomeRejected}
	}

	versions := dedupeStrings(filterExactVersions(parseMiseToolsTypescriptVersions(string(data))))
	switch len(versions) {
	case 0:
		return compilerOriginOutcome{state: compilerOutcomeEmpty}
	case 1:
		return compilerOriginOutcome{state: compilerOutcomePass, version: versions[0], origin: compilerOriginMiseProject}
	default:
		return compilerOriginOutcome{state: compilerOutcomeConflict, rootFindings: []ReadinessRootFinding{{Root: "."}}}
	}
}

func filterExactVersions(values []string) []string {
	exact := make([]string, 0, len(values))
	for _, value := range values {
		if isExactVersion(value) {
			exact = append(exact, value)
		}
	}
	return exact
}

// parseMiseToolsTypescriptVersions extracts every value assigned to the
// npm:typescript key inside a mise.toml [tools] table, tolerating both a
// single quoted string and a quoted-string array. It is a narrow, line-based
// scan rather than a full TOML parser: any other section, including [hooks]
// and [tasks], is skipped unread.
func parseMiseToolsTypescriptVersions(data string) []string {
	var versions []string
	inTools := false
	for _, rawLine := range strings.Split(data, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inTools = strings.HasPrefix(line, miseToolsSectionHeader)
			continue
		}
		if !inTools {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.Trim(strings.TrimSpace(key), `"'`) != "npm:typescript" {
			continue
		}
		versions = append(versions, parseMiseToolValue(strings.TrimSpace(value))...)
	}
	return versions
}

func parseMiseToolValue(value string) []string {
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
		var values []string
		for _, part := range strings.Split(inner, ",") {
			if unquoted, ok := unquoteMiseString(strings.TrimSpace(part)); ok {
				values = append(values, unquoted)
			}
		}
		return values
	}
	if unquoted, ok := unquoteMiseString(value); ok {
		return []string{unquoted}
	}
	return nil
}

func unquoteMiseString(value string) (string, bool) {
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return value[1 : len(value)-1], true
	}
	return "", false
}

const (
	miseProbeTimeout   = 10 * time.Second
	maxMiseProbeOutput = 4 << 10
)

// detectGlobalMiseTypescriptVersion is the global-mise detection seam: it
// shells out to a read-only `mise config get` invocation -- never `mise
// install` or any other mutating subcommand -- to report the npm:typescript
// version pinned in the user's global mise configuration. A non-zero exit
// means the key is unset in the global config: no candidate, not a probe
// failure. Tests may replace it to exercise mise_global-only resolution
// without depending on the host's real global mise installation.
var detectGlobalMiseTypescriptVersion = func(ctx context.Context) (version string, found bool, err error) {
	if _, lookErr := exec.LookPath("mise"); lookErr != nil {
		return "", false, nil
	}

	data, _, probeErr := runBoundedSubprocessProbeAt(ctx, miseProbeTimeout, maxMiseProbeOutput, miseProbeWorkingDir(), miseProbeEnv(), "mise", "config", "get", "tools.npm:typescript", "-g")
	if probeErr != nil {
		return "", false, probeErr
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "", false, nil
	}
	return trimmed, true, nil
}

// resolveMiseGlobalCompiler is compiler-resolution origin 3: global mise
// configuration. A probe failure (mise absent, timeout, unparsable output)
// is treated as no candidate rather than an operational error: this is the
// last origin in the frozen precedence, and its absence alone must still
// resolve to the typescript_compiler_missing gap, not a crash or hang.
func resolveMiseGlobalCompiler() compilerOriginOutcome {
	version, found, err := detectGlobalMiseTypescriptVersion(context.Background())
	if err != nil || !found || !isExactVersion(version) {
		return compilerOriginOutcome{state: compilerOutcomeEmpty}
	}
	return compilerOriginOutcome{state: compilerOutcomePass, version: version, origin: compilerOriginMiseGlobal}
}

func miseProbeWorkingDir() string {
	dir := filepath.Join(os.TempDir(), "coach-mise-probe")
	_ = os.MkdirAll(dir, 0o755)
	return dir
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

const (
	miseWhereProbeTimeout   = 10 * time.Second
	maxMiseWhereProbeOutput = 4 << 10
)

// locateMiseTypescriptInstall is the mise-install-location seam: it shells
// out to a read-only `mise where npm:typescript@<version>` -- never `mise
// install` or any other mutating subcommand, per this package's hazards/
// rejected-configuration rule -- to find where mise actually installed that
// exact version. A declared-but-never-installed version (the actual `mise
// install` step is out of this package's scope) fails this probe;
// resolveCompilerForRuntime treats that as "this origin contributes
// nothing," never as an operational error. `mise where` reports the tool's
// install root, not the npm package directory itself: the actual
// `typescript` package -- confirmed by checking for its package.json --
// lives at <install-root>/node_modules/typescript.
//
// Read-only mise probes (`mise config get` and `mise where`) run with a
// neutral working directory and a minimal environment (PATH, HOME, and
// mise's data/config directories) so the analyzed repository's own
// mise.toml — including env templates that execute commands — is never
// loaded during readiness or analysis.
var locateMiseTypescriptInstall = func(ctx context.Context, version string) (string, bool) {
	if _, lookErr := exec.LookPath("mise"); lookErr != nil {
		return "", false
	}

	data, exitErr, probeErr := runBoundedSubprocessProbeAt(ctx, miseWhereProbeTimeout, maxMiseWhereProbeOutput, miseProbeWorkingDir(), miseProbeEnv(), "mise", "where", "npm:typescript@"+version)
	if probeErr != nil || exitErr != nil {
		return "", false
	}

	toolRoot := strings.TrimSpace(string(data))
	if toolRoot == "" {
		return "", false
	}
	pkgDir := filepath.Join(toolRoot, "node_modules", "typescript")
	if _, statErr := os.Stat(filepath.Join(pkgDir, "package.json")); statErr != nil {
		return "", false
	}
	return pkgDir, true
}
