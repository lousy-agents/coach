package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

const (
	projectConfigMigrationPrompt         = "Copy project.json to .lousy-agents/coach.json? Coach will not delete project.json. [y/N] "
	projectConfigMigrationCopied         = "Copied project.json to .lousy-agents/coach.json. Commit that file so the next scan reads it. project.json was left in place.\n"
	projectConfigMigrationSkippedMessage = "project.json can be moved to .lousy-agents/coach.json. Next: top=\"$(git rev-parse --show-toplevel)\" && mkdir -p \"$top\"/.lousy-agents && git -C \"$top\" mv -- project.json .lousy-agents/coach.json"
)

func prepareScanProjectAnalysis(dir, revision string, f codesignalFlags, status *scanStatus, view *codesignalcli.WorktreeView, stdin, stderr *os.File, ports scanPorts) (*codesignalcli.ProjectAnalysis, []codesignal.Diagnostic, error) {
	suppressUncommitted, migrationDiags := offerDeprecatedProjectConfigMigration(dir, revision, f, status, stdin, stderr, ports)
	prepared, err := prepareProjectAnalysis(dir, revision, f.namedPolicy, f.projectLanguage, suppressUncommitted, ports)
	if err != nil {
		return nil, nil, err
	}
	project := prepared.analysis
	if view != nil && project == nil {
		project = &codesignalcli.ProjectAnalysis{}
	}
	if project != nil {
		project.Worktree = view
		project.Status = status.get()
	}
	return project, append(prepared.diagnostics, migrationDiags...), nil
}

func offerDeprecatedProjectConfigMigration(dir, revision string, f codesignalFlags, status *scanStatus, stdin, stderr *os.File, ports scanPorts) (bool, []codesignal.Diagnostic) {
	if !deprecatedProjectConfigCanMigrate(dir, revision, f, ports) {
		return false, nil
	}
	if !terminal.HasControllingTerminal(stdin) || nonInteractiveRequested(f) {
		return false, []codesignal.Diagnostic{projectConfigMigrationSkipped()}
	}
	fmt.Fprint(stderr, projectConfigMigrationPrompt)
	if !answeredYes(stdin) {
		return false, nil
	}
	return acceptProjectConfigMigrationCopy(dir, stderr, status)
}

// acceptProjectConfigMigrationCopy copies project.json, which leaves a new file that is untracked until it is
// committed. The status is read before the copy, so that file is never counted as the customer's own dirt.
func acceptProjectConfigMigrationCopy(dir string, stderr *os.File, status *scanStatus) (bool, []codesignal.Diagnostic) {
	status.get()
	if err := copyWorktreeProjectConfig(dir); err != nil {
		return false, []codesignal.Diagnostic{projectConfigMigrationFailed(err)}
	}
	fmt.Fprint(stderr, projectConfigMigrationCopied)
	return true, nil
}

func deprecatedProjectConfigCanMigrate(dir, revision string, f codesignalFlags, ports scanPorts) bool {
	if f.namedPolicy.Explicit {
		return false
	}
	selection, err := ports.discover(dir, revision)
	return err == nil && selection.Fatal == nil && selection.Deprecated && !selection.CanonicalPathTaken()
}

func projectConfigMigrationSkipped() codesignal.Diagnostic {
	return codesignal.Diagnostic{
		Kind:    "project_config_migration_skipped",
		Path:    projectconfig.DefaultPath,
		Message: projectConfigMigrationSkippedMessage,
	}
}

func projectConfigMigrationFailed(err error) codesignal.Diagnostic {
	return codesignal.Diagnostic{
		Kind:    "project_config_migration_failed",
		Path:    projectconfig.CanonicalPath,
		Message: fmt.Sprintf("Could not copy project.json to .lousy-agents/coach.json (%s). Left both files unchanged.", err),
	}
}
