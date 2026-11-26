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
func withEmptyRootsJudged(coverage projectmodel.Coverage, roots []string) projectmodel.Coverage {
	if coverage.Counts["files_seen"] != 0 {
		return coverage
	}

	judged := coverage
	judged.Complete = false
	judged.Diagnostics = append(slices.Clone(coverage.Diagnostics), projectmodel.Diagnostic{
		Code: projectmodel.DiagRootsEmpty,
		Message: fmt.Sprintf("the policy's roots [%s] hold no Go source: roots_seen=%d, files_seen=%d; the Go project analysis saw nothing",
			strings.Join(roots, ", "), coverage.Counts["roots_seen"], coverage.Counts["files_seen"]),
	})
	return domain.CanonicalCoverage(judged)
}
