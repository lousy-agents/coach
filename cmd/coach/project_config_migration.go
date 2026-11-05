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
	projectConfigMigrationSkippedMessage = "project.json can be moved to .lousy-agents/coach.json. Next: mkdir -p .lousy-agents && git mv project.json .lousy-agents/coach.json"
)

func prepareScanProjectAnalysis(dir, revision string, f codesignalFlags, status codesignalcli.WorktreeStatus, view *codesignalcli.WorktreeView, stdin, stderr *os.File) (*codesignalcli.ProjectAnalysis, []codesignal.Diagnostic, error) {
	suppressUncommitted, migrationDiags := offerDeprecatedProjectConfigMigration(dir, revision, f, stdin, stderr)
	project, diags, err := prepareProjectAnalysis(dir, revision, f.projectConfigSet, f.projectConfig, f.projectLanguage, suppressUncommitted)
	if err != nil {
		return nil, nil, err
	}
	if view != nil && project == nil {
		project = &codesignalcli.ProjectAnalysis{}
	}
	if project != nil {
		project.Worktree = view
		project.Status = status
	}
	return project, append(diags, migrationDiags...), nil
}

func offerDeprecatedProjectConfigMigration(dir, revision string, f codesignalFlags, stdin, stderr *os.File) (bool, []codesignal.Diagnostic) {
	if !deprecatedProjectConfigCanMigrate(dir, revision, f) {
		return false, nil
	}
	if !terminal.HasControllingTerminal(stdin) || nonInteractiveRequested(f) {
		return false, []codesignal.Diagnostic{projectConfigMigrationSkipped()}
	}
	fmt.Fprint(stderr, projectConfigMigrationPrompt)
	if !answeredYes(stdin) {
		return false, nil
	}
	return acceptProjectConfigMigrationCopy(dir, stderr)
}

func acceptProjectConfigMigrationCopy(dir string, stderr *os.File) (bool, []codesignal.Diagnostic) {
	if err := copyWorktreeProjectConfig(dir); err != nil {
		return false, []codesignal.Diagnostic{projectConfigMigrationFailed(err)}
	}
	fmt.Fprint(stderr, projectConfigMigrationCopied)
	return true, nil
}

func deprecatedProjectConfigCanMigrate(dir, revision string, f codesignalFlags) bool {
	if f.projectConfigSet {
		return false
	}
	selection, err := discoverProjectConfig(dir, revision)
	return err == nil && selection.Fatal == nil && selection.Deprecated && !selection.UncommittedCanonical
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
