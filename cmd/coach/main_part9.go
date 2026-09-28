package main

import (
	"context"

	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"

	"os"
	"sort"
	"strconv"
)

func runDiffAnalysis(dir string, f codesignalFlags, stderr *os.File) (*codesignal.Report, error) {
	headSHA, mergeBaseSHA, err := codesignalcli.ResolveRevisions(dir, f.base)
	if err != nil {
		return nil, err
	}

	selected, diagnostics, err := codesignalcli.SelectChangedFiles(dir, mergeBaseSHA)
	if err != nil {
		return nil, err
	}
	selected, excluded, err := codesignalcli.ApplySourceScope(dir, headSHA, f.buildTarget, f.scope, selected)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, codesignalcli.WorkingTreeDisclosureDiagnostics(dir)...)

	project, diag, opErr := prepareProjectAnalysis(dir, headSHA, f.projectConfigSet, f.projectConfig, f.projectLanguage)
	if opErr != nil {
		return nil, opErr
	}
	report, err := codesignalcli.AnalyzeChanges(context.Background(), dir, headSHA, mergeBaseSHA, selected, diagnostics, f.scope, excluded, project)
	if err != nil {
		return nil, wrapScanAnalysisError(err, dir, headSHA, f.projectConfig, stderr)
	}
	return withProjectDiagnostic(report, diag), nil
}
func (c *countingBoolFlag) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	c.value = v
	c.count++
	return nil
}
func sortedFlagNames(setFlags map[string]bool) []string {
	names := make([]string, 0, len(setFlags))
	for name := range setFlags {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func (c *countingStringFlag) String() string {
	if c == nil {
		return ""
	}
	return c.value
}
func rejectPositionalArgs(flagName string, positional []string, suffix string) string {
	if len(positional) == 0 {
		return ""
	}
	return fmt.Sprintf("coach: --%s does not accept positional arguments%s", flagName, suffix)
}
