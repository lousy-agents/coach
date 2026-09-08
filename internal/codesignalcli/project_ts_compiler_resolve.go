package codesignalcli

import (
	"regexp"
	"strings"
	"time"
)

const (
	compilerOriginProject             = "project"
	compilerOriginMiseProject         = "mise_project"
	compilerOriginMiseGlobal          = "mise_global"
	compilerDeclarationOriginManifest = "manifest"
)

// SupportedTypescriptVersions is the compiled-in set of exact TypeScript
// compiler versions this build will load, ascending. TypeScript does not
// follow semantic versioning and the analyzer depends on
// typescript/unstable/* subpaths, so no range is promised.
var SupportedTypescriptVersions = []string{"7.0.2"}

func newestSupportedTypescriptVersion() string {
	return SupportedTypescriptVersions[len(SupportedTypescriptVersions)-1]
}

func isSupportedTypescriptVersion(version string) bool {
	for _, candidate := range SupportedTypescriptVersions {
		if candidate == version {
			return true
		}
	}
	return false
}

func supportedTypescriptVersionsCopy() []string {
	out := make([]string, len(SupportedTypescriptVersions))
	copy(out, SupportedTypescriptVersions)
	return out
}

var exactVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

func isExactVersion(value string) bool {
	return exactVersionPattern.MatchString(strings.TrimSpace(value))
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func resolveCompiler(dir string, roots []string) ReadinessCheck {
	return compilerCheckFromAggregate(evaluateCompilerOrigins(dir, roots))
}

const (
	compilerWorktreeRootTimeout   = 10 * time.Second
	maxCompilerWorktreeRootOutput = 4 << 10
	maxCompilerWorktreeRootStderr = 4 << 10
)

// compilerWorktreeRoot is the walk ceiling for nearest-package.json
// resolution, not the project-manifest origin. CheckProjectReadiness has
// already failed closed on an unreadable worktree by the time this runs, so
// a resolution failure here falls back to dir rather than blocking the
// compiler check.
func compilerWorktreeRoot(dir string) string {
	output, err := runGitBytesBounded(dir, maxCompilerWorktreeRootOutput, maxCompilerWorktreeRootStderr, compilerWorktreeRootTimeout, "rev-parse", "--show-toplevel")
	if err != nil {
		return dir
	}
	root := strings.TrimSpace(string(output))
	if root == "" {
		return dir
	}
	return root
}

type compilerRuntimeResolution struct {
	Origin            string
	Version           string
	Path              string
	NativePackagePath string
}

// CompilerUnresolvedError is the typed scan-time failure the CLI maps to
// exit 2 with one stderr remediation line. A host-runtime probe failure
// that is not a clean miss stays an operational error, not this type.
// RootFindings entries are repository-relative policy roots, never
// filesystem paths.
type CompilerUnresolvedError struct {
	Code         string
	ConfigPath   string
	RootFindings []ReadinessRootFinding
}

func (e *CompilerUnresolvedError) Error() string {
	return e.RemediationLine()
}

func (e *CompilerUnresolvedError) RemediationLine() string {
	invocation := "coach codesignal --baseline --check-project --project-language typescript"
	if e.ConfigPath != "" {
		invocation += " --project-config " + e.ConfigPath
	}
	return e.Code + formatRemediationRootFindings(e.RootFindings) + ": run " + invocation
}

func formatRemediationRootFindings(findings []ReadinessRootFinding) string {
	formatted := formatRootFindings(findings)
	if formatted == "" {
		return ""
	}
	return " (" + formatted + ")"
}

func compilerUnresolved(code string) *CompilerUnresolvedError {
	return &CompilerUnresolvedError{Code: code}
}

func resolveCompilerForRuntime(dir string, roots []string) (compilerRuntimeResolution, error) {
	aggregate := evaluateCompilerOrigins(dir, roots)
	code, findings := compilerOutcomeFromAggregate(aggregate)
	if code != "" {
		return compilerRuntimeResolution{}, &CompilerUnresolvedError{Code: code, RootFindings: findings}
	}
	return compilerRuntimeResolution{
		Origin:            aggregate.winner.origin,
		Version:           aggregate.winner.version,
		Path:              aggregate.winner.path,
		NativePackagePath: aggregate.winner.nativePath,
	}, nil
}
