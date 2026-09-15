package codesignalcli

import "testing"

func TestAppendedRemediationLine(t *testing.T) {
	const line = "coach codesignal --baseline --prepare-compiler --project-language typescript"

	if got := AppendedRemediationLine(false, line); got != line {
		t.Fatalf("AppendedRemediationLine(false, line) = %q, want %q", got, line)
	}
	if got := AppendedRemediationLine(true, line); got != "" {
		t.Fatalf("AppendedRemediationLine(true, line) = %q, want empty: the interactive setup offer (#330 Task 7) owns the controlling-terminal case", got)
	}
}

func TestPrepareCompilerRemediation(t *testing.T) {
	if got, want := PrepareCompilerRemediation(GapTypescriptCompilerMissing, "project.json"), "coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want %q", GapTypescriptCompilerMissing, "project.json", got, want)
	}
	if got, want := PrepareCompilerRemediation(GapTypescriptVersionMismatch, ""), "coach codesignal --baseline --prepare-compiler --project-language typescript"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, \"\") = %q, want %q", GapTypescriptVersionMismatch, got, want)
	}
	if got, want := PrepareCompilerRemediation(GapTypescriptVersionConflict, "project.json"), "coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want %q", GapTypescriptVersionConflict, "project.json", got, want)
	}
	if got := PrepareCompilerRemediation(GapNodeMissing, "project.json"); got != "" {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want empty: Coach has no executable remediation for a runtime-boundary gap", GapNodeMissing, "project.json", got)
	}
	if got := PrepareCompilerRemediation(GapNodeUnsupported, "project.json"); got != "" {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want empty: Coach has no executable remediation for a runtime-boundary gap", GapNodeUnsupported, "project.json", got)
	}
}

func TestSuggestProjectConfigRemediation(t *testing.T) {
	if got, want := SuggestProjectConfigRemediation(), "coach codesignal --baseline --suggest-project-config --project-language typescript"; got != want {
		t.Fatalf("SuggestProjectConfigRemediation() = %q, want %q", got, want)
	}
}
