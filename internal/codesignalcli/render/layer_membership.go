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
// absent). One line describes the head revision; when the base revision also
// has files outside every layer, each revision gets its own named line,
// because the verdict's reach clause then describes the base as well.
func renderLayerMembershipLine(b *strings.Builder, membership *codesignal.LayerMembershipReport) {
	if membership == nil {
		return
	}
	base := membership.Base
	if base == nil || base.FilesOutsideLayers == 0 {
		writeLayerMembershipCount(b, membership, membership.Head)
		return
	}
	fmt.Fprintf(b, "Head revision %s: ", membership.Head.Revision)
	writeLayerMembershipCount(b, membership, membership.Head)
	fmt.Fprintf(b, "Base revision %s: ", base.Revision)
	writeLayerMembershipCount(b, membership, *base)
}

func writeLayerMembershipCount(b *strings.Builder, membership *codesignal.LayerMembershipReport, rev codesignal.LayerMembershipRevisionReport) {
	fmt.Fprintf(b, "%d of %d project files under the configured roots sit outside every declared layer", rev.FilesOutsideLayers, rev.AnalyzedFiles)
	if membership.KeyedByPackageDirectory {
		b.WriteString(" (a Go file is in a layer when its package directory is)")
	}
	if rev.FilesOutsideLayers > 0 && membership.ForbiddenImportsDeclared {
		b.WriteString("; imports to or from them are not evaluated against forbidden_imports")
	}
	b.WriteString(".\n")
}

const forbiddenImportsReachClause = "forbidden_imports was evaluated only between files counted inside declared layers"

// hasFilesOutsideLayers is false when the policy declares no forbidden_imports,
// because the reach clause would then describe a rule that never ran.
func hasFilesOutsideLayers(report *codesignal.Report) bool {
	membership := report.LayerMembership
	if membership == nil || !membership.ForbiddenImportsDeclared {
		return false
	}
	return membership.Head.FilesOutsideLayers > 0 ||
		(membership.Base != nil && membership.Base.FilesOutsideLayers > 0)
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
