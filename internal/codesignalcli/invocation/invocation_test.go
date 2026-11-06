package invocation

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

func TestCommandText(t *testing.T) {
	cases := []struct {
		name    string
		command Command
		want    string
	}{
		{"the check for a discovered policy", Command{Mode: CheckProject}, "coach codesignal --baseline --check-project --project-language typescript"},
		{"the check for a named policy", Command{Mode: CheckProject, Policy: policyref.Named("project.json")}, "coach codesignal --baseline --check-project --project-language typescript --project-config project.json"},
		{"compiler setup for a named policy", Command{Mode: PrepareCompiler, Policy: policyref.Named("project.json")}, "coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"},
		{"the scan for a named policy", Command{Mode: Scan, Policy: policyref.Named("project.json")}, "coach codesignal --baseline --project-config project.json --project-language typescript"},
		{"the scan for a discovered policy", Command{Mode: Scan}, "coach codesignal --baseline --project-language typescript"},
		{"the scan for a discovered policy whose path is known", Command{Mode: Scan, Policy: policyref.Discovered("project.json")}, "coach codesignal --baseline --project-language typescript"},
		{"the check for a discovered policy whose path is known", Command{Mode: CheckProject, Policy: policyref.Discovered("project.json")}, "coach codesignal --baseline --check-project --project-language typescript"},
		{"guided authoring", Command{Mode: SuggestProjectConfig}, "coach codesignal --baseline --suggest-project-config --project-language typescript"},
		{"a path a shell would expand", Command{Mode: CheckProject, Policy: policyref.Named("$(touch PWNED).json")}, `coach codesignal --baseline --check-project --project-language typescript --project-config '$(touch PWNED).json'`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPhrase(t, c.command, "", c.want)
		})
	}
}

func assertPhrase(t *testing.T, command Command, verb, want string) {
	t.Helper()
	if got := command.Phrase(verb); got != want {
		t.Errorf("Phrase(%q) = %q; want %q", verb, got, want)
	}
}

func TestCommandPhraseLeadsWithTheVerbWhenThereIsOne(t *testing.T) {
	command := Command{Mode: CheckProject}

	const bare = "coach codesignal --baseline --check-project --project-language typescript"
	if got, want := command.Phrase("run"), "run "+bare; got != want {
		t.Errorf("Phrase(run) = %q; want %q", got, want)
	}
	if got := command.Phrase(""); got != bare {
		t.Errorf("Phrase(\"\") = %q; want the bare command %q", got, bare)
	}
}

func TestCommandPhraseNamesNoCommandForAPolicyWhoseNameCannotBePrinted(t *testing.T) {
	const unprintable = "bad\x1bname.json"
	cases := []struct {
		name string
		mode Mode
		verb string
		want string
	}{
		{"the check", CheckProject, "run", "check it with --check-project and the same --project-config (its name is not printed here)"},
		{"compiler setup", PrepareCompiler, "", "run --prepare-compiler with the same --project-config (its name is not printed here)"},
		{"the scan", Scan, "", "rerun this scan with the same --project-config (its name is not printed here)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPhrase(t, Command{Mode: c.mode, Policy: policyref.Named(unprintable)}, c.verb, c.want)
		})
	}
}
