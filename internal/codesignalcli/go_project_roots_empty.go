package codesignalcli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// withEmptyRootsJudged marks the head side of a Go project analysis
// incomplete when it analysed no file under the policy's roots. The base side
// is not judged: a diff that adds the project has no Go source before it.
// Coverage already incomplete for another cause is left alone, because a zero
// count then says discovery stopped early, not that the roots hold no Go source.
func withEmptyRootsJudged(coverage projectmodel.Coverage, roots []string) projectmodel.Coverage {
	if !coverage.Complete || coverage.Counts["files_seen"] != 0 {
		return coverage
	}

	return withRootsEmptyDiagnostic(coverage, fmt.Sprintf("the policy's roots [%s] hold no Go source: roots_seen=%d, files_seen=%d; the Go project analysis saw nothing",
		strings.Join(roots, ", "), coverage.Counts["roots_seen"], coverage.Counts["files_seen"]))
}

// withEmptyTSProjectsJudged does the same for the head side of a TypeScript
// project analysis, where the sidecar reports a complete analysis that opened
// no project when the roots hold no tsconfig.json and no TypeScript source. A
// coverage that is already incomplete keeps its own cause.
func withEmptyTSProjectsJudged(coverage projectmodel.Coverage, roots []string) projectmodel.Coverage {
	if !coverage.Complete || coverage.Counts["tsconfig_count"] != 0 {
		return coverage
	}

	return withRootsEmptyDiagnostic(coverage, fmt.Sprintf("the policy's roots [%s] hold no TypeScript project: tsconfig_count=%d, files_seen=%d; the TypeScript project analysis saw nothing",
		strings.Join(roots, ", "), coverage.Counts["tsconfig_count"], coverage.Counts["files_seen"]))
}

func withRootsEmptyDiagnostic(coverage projectmodel.Coverage, message string) projectmodel.Coverage {
	judged := coverage
	judged.Complete = false
	judged.Diagnostics = append(slices.Clone(coverage.Diagnostics), projectmodel.Diagnostic{
		Code:    projectmodel.DiagRootsEmpty,
		Message: message,
	})
	return domain.CanonicalCoverage(judged)
}
