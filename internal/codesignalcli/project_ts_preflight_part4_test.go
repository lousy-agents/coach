package codesignalcli

import (
	"errors"

	"testing"
)

// TestPrepareCompilerRemediationWithReadinessWithholdsADeadEndCommand pins
// O2: even for a gap code whose next action is the executable
// prepare-compiler kind, the command must be withheld unless the attached
// readiness snapshot offers a choice --prepare-compiler itself can execute.
// That flag runs mise scopes only (filterMiseChoiceKinds), so a menu whose
// sole executable entry is project_package is as much a dead end as an empty
// one: the named command would open, discard the only offered choice, and
// exit 0 reporting it had nothing to set up while the compiler is still
// missing.
func TestPrepareCompilerRemediationWithReadinessWithholdsADeadEndCommand(t *testing.T) {
	nothingOffered := &ReadinessResult{
		Checks: ReadinessChecks{Compiler: ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing}},
	}
	if got := PrepareCompilerRemediationWithReadiness(GapTypescriptCompilerMissing, "project.json", nothingOffered); got != "" {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(dead-end menu) = %q, want empty", got)
	}

	projectPackageOnly := &ReadinessResult{
		Checks: ReadinessChecks{
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}
	if got := PrepareCompilerRemediationWithReadiness(GapTypescriptCompilerMissing, "project.json", projectPackageOnly); got != "" {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(project_package-only menu) = %q, want empty: --prepare-compiler runs mise scopes only, so it would exit 0 having set nothing up", got)
	}

	miseOffered := &ReadinessResult{
		Checks:      ReadinessChecks{Compiler: ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing}},
		MiseChoices: []ReadinessMiseChoice{{Kind: compilerOriginMiseProject, Verified: true}},
	}
	want := "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"
	if got := PrepareCompilerRemediationWithReadiness(GapTypescriptCompilerMissing, "project.json", miseOffered); got != want {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(verified mise scope) = %q, want %q", got, want)
	}

	if got := PrepareCompilerRemediationWithReadiness(GapNodeMissing, "project.json", miseOffered); got != "" {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(%q, ...) = %q, want empty: a runtime-boundary gap code must still be withheld regardless of readiness", GapNodeMissing, got)
	}

	if got, want := PrepareCompilerRemediationWithReadiness(GapTypescriptCompilerMissing, "project.json", nil), PrepareCompilerRemediation(GapTypescriptCompilerMissing, "project.json"); got != want {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(nil readiness) = %q, want the plain PrepareCompilerRemediation fallback %q", got, want)
	}
}

// TestWrapCompilerUnresolvedErrorWithReadinessUnwrapsToThePlainError pins
// classifyAnalysisError's own no-controlling-terminal fallback: it locates a
// *CompilerUnresolvedError via errors.As without any change, because
// CompilerUnresolvedErrorWithReadiness.Unwrap returns the original value
// unchanged.
func TestWrapCompilerUnresolvedErrorWithReadinessUnwrapsToThePlainError(t *testing.T) {
	repo := newTempGitRepoT(t)
	head := commitFileT(t, repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

	original := &CompilerUnresolvedError{Code: GapTypescriptCompilerMissing, ConfigPath: "project.json"}
	wrapped := WrapCompilerUnresolvedErrorWithReadiness(original, repo, head, "project.json")

	var withReadiness *CompilerUnresolvedErrorWithReadiness
	if !errors.As(wrapped, &withReadiness) {
		t.Fatalf("WrapCompilerUnresolvedErrorWithReadiness did not produce a *CompilerUnresolvedErrorWithReadiness: %v", wrapped)
	}
	if withReadiness.Readiness == nil {
		t.Fatalf("CompilerUnresolvedErrorWithReadiness.Readiness is nil, want a populated snapshot")
	}
	if withReadiness.Revision != head || withReadiness.ConfigPath != "project.json" {
		t.Fatalf("CompilerUnresolvedErrorWithReadiness = {Revision: %q, ConfigPath: %q}, want {%q, %q}", withReadiness.Revision, withReadiness.ConfigPath, head, "project.json")
	}

	var plain *CompilerUnresolvedError
	if !errors.As(wrapped, &plain) {
		t.Fatalf("errors.As could not unwrap back to the original *CompilerUnresolvedError")
	}
	if plain != original {
		t.Fatalf("errors.As unwrapped to a different *CompilerUnresolvedError value than the one passed in")
	}
}

func TestWrapProjectConfigErrorWithReadinessLeavesOtherErrorsUnchanged(t *testing.T) {
	plain := &OperationalError{Message: "boom"}
	if got := WrapProjectConfigErrorWithReadiness(plain, ".", "HEAD", "project.json"); got != error(plain) {
		t.Fatalf("WrapProjectConfigErrorWithReadiness(non-ProjectConfigError) = %v, want the original error unchanged", got)
	}
}
