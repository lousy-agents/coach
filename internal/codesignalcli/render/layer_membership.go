package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// renderLayerMembershipLine states how many project files under the configured
// roots sit outside every declared layer, directly above the verdict, so a
// clean verdict is read next to the reach of the layer rules. The population is
// the project model's files, which is not the file-local set the summary line
// counts: code outside the roots is not in this count, but the summary may
// still include it. It always prints the count when the block is present,
// including zero, so a reader can tell a real zero from "not computed" (block
// absent). The line describes the head revision.
func renderLayerMembershipLine(b *strings.Builder, membership *codesignal.LayerMembershipReport) {
	if membership == nil {
		return
	}
	head := membership.Head
	fmt.Fprintf(b, "%d of %d project files under the configured roots sit outside every declared layer", head.FilesOutsideLayers, head.AnalyzedFiles)
	if head.FilesOutsideLayers > 0 {
		b.WriteString("; imports to or from them are not evaluated against forbidden_imports")
	}
	b.WriteString(".\n")
}

const forbiddenImportsReachClause = "forbidden_imports was evaluated only between files inside declared layers"

func hasFilesOutsideLayers(report *codesignal.Report) bool {
	return report.LayerMembership != nil && report.LayerMembership.Head.FilesOutsideLayers > 0
}

// layerReachSuffix scopes an all-clear to what the forbidden_imports check
// could see, in the same terms as the layer-membership line above the
// verdict. It is empty when nothing sits outside a layer or the count was not
// computed, so those verdicts are unchanged.
func layerReachSuffix(report *codesignal.Report) string {
	if !hasFilesOutsideLayers(report) {
		return ""
	}
	return "; " + forbiddenImportsReachClause
}

// layerReachSentence is the same scoping as a sentence of its own, for a
// verdict that is already qualified by "but ..." and so cannot take a second
// "; " clause without mixing a by-design limit into the failure causes.
func layerReachSentence(report *codesignal.Report) string {
	if !hasFilesOutsideLayers(report) {
		return ""
	}
	return " Separately, " + forbiddenImportsReachClause + "."
}
