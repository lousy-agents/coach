package main

import (
	"fmt"
	"os"
	"strings"

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

func prepareScanProjectAnalysis(dir, revision string, f codesignalFlags, stdin, stderr *os.File) (*codesignalcli.ProjectAnalysis, []codesignal.Diagnostic, func(), error) {
	suppressUncommitted, migrationDiags, restore := offerDeprecatedProjectConfigMigration(dir, revision, f, stdin, stderr)
	project, diags, err := prepareProjectAnalysis(dir, revision, f.projectConfigSet, f.projectConfig, f.projectLanguage, suppressUncommitted)
	if err != nil {
		return nil, nil, restore, err
	}
	return project, append(diags, migrationDiags...), restore, nil
}

func offerDeprecatedProjectConfigMigration(dir, revision string, f codesignalFlags, stdin, stderr *os.File) (bool, []codesignal.Diagnostic, func()) {
	if !deprecatedProjectConfigCanMigrate(dir, revision, f) {
		return false, nil, leaveWorktreeStatusLive
	}
	if !terminal.HasControllingTerminal(stdin) || nonInteractiveRequested(f) {
		return false, []codesignal.Diagnostic{projectConfigMigrationSkipped()}, leaveWorktreeStatusLive
	}
	fmt.Fprint(stderr, projectConfigMigrationPrompt)
	if !answeredYes(stdin) {
		return false, nil, leaveWorktreeStatusLive
	}
	return acceptProjectConfigMigrationCopy(dir, stderr)
}

func acceptProjectConfigMigrationCopy(dir string, stderr *os.File) (bool, []codesignal.Diagnostic, func()) {
	entries, statusErr := codesignalcli.CaptureWorktreeStatus(dir)
	if err := copyWorktreeProjectConfig(dir); err != nil {
		return false, []codesignal.Diagnostic{projectConfigMigrationFailed(err)}, leaveWorktreeStatusLive
	}
	fmt.Fprint(stderr, projectConfigMigrationCopied)
	return true, nil, codesignalcli.UseWorktreeStatusSnapshot(entries, statusErr)
}

func leaveWorktreeStatusLive() {}

func deprecatedProjectConfigCanMigrate(dir, revision string, f codesignalFlags) bool {
	if f.projectConfigSet {
		return false
	}
	selection, err := discoverProjectConfig(dir, revision)
	return err == nil && selection.Fatal == nil && selection.Deprecated && !selection.UncommittedCanonical
}

func answeredYes(stdin *os.File) bool {
	answer := strings.ToLower(strings.TrimSpace(readLineWithoutReadAhead(stdin)))
	return answer == "y" || answer == "yes"
}

// readLineWithoutReadAhead takes one byte at a time because a buffered
// reader would swallow answers typed ahead for the next prompt, which reads
// stdin again.
func readLineWithoutReadAhead(stdin *os.File) string {
	var line []byte
	next := make([]byte, 1)
	for {
		n, err := stdin.Read(next)
		if n == 1 {
			if next[0] == '\n' {
				break
			}
			line = append(line, next[0])
		}
		if err != nil {
			break
		}
	}
	return string(line)
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
