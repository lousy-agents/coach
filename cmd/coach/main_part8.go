package main

import (
	"context"

	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"

	"os"
)

func runBaselineAnalysis(dir string, f codesignalFlags, stderr *os.File) (*codesignal.Report, error) {
	revisionSHA, err := codesignalcli.ResolveBaselineRevision(dir)
	if err != nil {
		return nil, err
	}
	discovered, coverage, err := codesignalcli.DiscoverTrackedFiles(dir, revisionSHA)
	if err != nil {
		return nil, err
	}
	kept, excluded, err := codesignalcli.ApplyBaselineSourceScope(dir, revisionSHA, f.buildTarget, f.scope, discovered)
	if err != nil {
		return nil, err
	}
	coverage.Excluded = excluded

	project, diag, opErr := prepareProjectAnalysis(dir, revisionSHA, f.projectConfigSet, f.projectConfig, f.projectLanguage)
	if opErr != nil {
		return nil, opErr
	}
	disclosure := codesignalcli.WorkingTreeDisclosureDiagnostics(dir)
	report, err := codesignalcli.AnalyzeBaseline(context.Background(), dir, revisionSHA, kept, disclosure, f.scope, coverage, project)
	if err != nil {
		return nil, wrapScanAnalysisError(err, dir, revisionSHA, f.projectConfig, stderr)
	}
	return withProjectDiagnostic(report, diag), nil
}
func validateSuggestProjectConfigFlags(f codesignalFlags, setFlags map[string]bool, positional []string, suggestCount, outputCount int) string {
	const suffix = " (project_config_suggestion_invalid_arguments)"
	if suggestCount > 1 {
		return "coach: --suggest-project-config may only be provided once" + suffix
	}
	if outputCount > 1 {
		return "coach: --output may only be provided once" + suffix
	}
	if !f.baseline {
		return "coach: --suggest-project-config requires --baseline" + suffix
	}
	allowedWithSuggest := map[string]bool{"suggest-project-config": true, "output": true, "baseline": true}
	if f.projectLanguage == "typescript" {

		allowedWithSuggest["project-language"] = true
		allowedWithSuggest["no-interactive"] = true
	}
	if name, disallowed := firstDisallowedFlag(setFlags, allowedWithSuggest); disallowed {
		return fmt.Sprintf("coach: --suggest-project-config cannot be combined with --%s%s", name, suffix)
	}
	return rejectPositionalArgs("suggest-project-config", positional, suffix)
}
