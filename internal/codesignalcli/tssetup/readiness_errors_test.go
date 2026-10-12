package tssetup

import (
	"errors"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// TestWrapCompilerUnresolvedErrorWithReadinessUnwrapsToThePlainError pins
// classifyAnalysisError's own no-controlling-terminal fallback: it locates a
// *tstoolchain.CompilerUnresolvedError via errors.As without any change, because
// CompilerUnresolvedErrorWithReadiness.Unwrap returns the original value
// unchanged.
func TestWrapCompilerUnresolvedErrorWithReadinessUnwrapsToThePlainError(t *testing.T) {
	repo := gitfixture.Init(t)
	head := gitfixture.CommitFile(t, repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

	original := &tstoolchain.CompilerUnresolvedError{Code: projectreadiness.GapTypescriptCompilerMissing, ConfigPath: "project.json"}
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

	var plain *tstoolchain.CompilerUnresolvedError
	if !errors.As(wrapped, &plain) {
		t.Fatalf("errors.As could not unwrap back to the original *CompilerUnresolvedError")
	}
	if plain != original {
		t.Fatalf("errors.As unwrapped to a different *CompilerUnresolvedError value than the one passed in")
	}
}

func TestWrapProjectConfigErrorWithReadinessLeavesOtherErrorsUnchanged(t *testing.T) {
	plain := &gitrepo.OperationalError{Message: "boom"}
	if got := WrapProjectConfigErrorWithReadiness(plain, ".", "HEAD", "project.json"); got != error(plain) {
		t.Fatalf("WrapProjectConfigErrorWithReadiness(non-ProjectConfigError) = %v, want the original error unchanged", got)
	}
}

func TestWrapCompilerUnresolvedErrorWithReadinessLeavesOtherErrorsUnchanged(t *testing.T) {
	plain := &gitrepo.OperationalError{Message: "boom"}
	if got := WrapCompilerUnresolvedErrorWithReadiness(plain, ".", "HEAD", "project.json"); got != error(plain) {
		t.Fatalf("WrapCompilerUnresolvedErrorWithReadiness(non-CompilerUnresolvedError) = %v, want the original error unchanged", got)
	}
}
