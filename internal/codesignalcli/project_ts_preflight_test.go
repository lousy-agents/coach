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
	if got, want := PrepareCompilerRemediation("project.json"), "coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q) = %q, want %q", "project.json", got, want)
	}
	if got, want := PrepareCompilerRemediation(""), "coach codesignal --baseline --prepare-compiler --project-language typescript"; got != want {
		t.Fatalf("PrepareCompilerRemediation(\"\") = %q, want %q", got, want)
	}
}

func TestSuggestProjectConfigRemediation(t *testing.T) {
	if got, want := SuggestProjectConfigRemediation(), "coach codesignal --baseline --suggest-project-config --project-language typescript"; got != want {
		t.Fatalf("SuggestProjectConfigRemediation() = %q, want %q", got, want)
	}
}
