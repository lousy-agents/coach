package codesignalcli

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func renderProjectProvenanceSection(b *strings.Builder, prov *codesignal.ProjectProvenance) {
	if prov == nil {
		return
	}
	b.WriteString("\nProject provenance:\n")
	fmt.Fprintf(b, "language: %s\n", prov.Language)
	fmt.Fprintf(b, "config_digest: %s\n", prov.ConfigDigest)
	fmt.Fprintf(b, "analyzer: %s %s (protocol %d)\n", prov.Analyzer.Version, prov.Analyzer.Digest, prov.Analyzer.ProtocolVersion)
	if len(prov.SelectedRoots) > 0 {
		fmt.Fprintf(b, "selected_roots: %s\n", strings.Join(prov.SelectedRoots, ", "))
	}
	fmt.Fprintf(b, "runtime: %s %s (%s)\n", prov.Runtime.Kind, prov.Runtime.Version, prov.Runtime.Origin)
	fmt.Fprintf(b, "compiler: %s (%s)\n", prov.Runtime.CompilerVersion, prov.Runtime.CompilerOrigin)
	renderPackageManagerLine(b, prov.PackageManager)
	renderProvenanceRevision(b, "head", prov.Head)
	if prov.Base != nil {
		renderProvenanceRevision(b, "base", *prov.Base)
	}
}

func renderPackageManagerLine(b *strings.Builder, pm *codesignal.ProvenancePackageManager) {
	if pm == nil {
		return
	}
	if pm.Version != "" {
		fmt.Fprintf(b, "package_manager: %s %s (%s)\n", pm.Kind, pm.Version, pm.Origin)
		return
	}
	fmt.Fprintf(b, "package_manager: %s (%s)\n", pm.Kind, pm.Origin)
}

func renderProvenanceRevision(b *strings.Builder, label string, rev codesignal.ProvenanceRevision) {
	fmt.Fprintf(b, "%s_revision: %s  model: %s  bypass: %s  reachability: %s\n",
		label, rev.Revision, rev.Coverage.Model, rev.Coverage.Bypass, rev.Coverage.Reachability)
}

func renderProjectNextActionsSection(b *strings.Builder, actions []codesignal.ProjectNextAction) {
	if len(actions) == 0 {
		return
	}
	b.WriteString("\nNext actions:\n")
	for _, action := range actions {
		renderNextAction(b, action)
	}
}

func renderNextAction(b *strings.Builder, action codesignal.ProjectNextAction) {
	switch action.Kind {
	case "record_baseline":
		fmt.Fprintf(b, "  record_baseline: retain the reported HEAD revision for a future `--base <revision>` comparison; Coach persists nothing\n")
	case "review_policy_coverage":
		fmt.Fprintf(b, "  review_policy_coverage: inspect selected roots and matched/unmatched layers against intent; revise the committed policy if they do not express it\n")
	case "inspect_diagnostics":
		fmt.Fprintf(b, "  inspect_diagnostics: resolve or explicitly accept reported limitations before trusting the absence of findings\n")
	default:
		fmt.Fprintf(b, "  %s\n", action.Kind)
	}
}
