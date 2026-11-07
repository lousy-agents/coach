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
		{"guided authoring into a file at the repository root", Command{Mode: SuggestProjectConfig, Target: "new.json"}, "coach codesignal --baseline --suggest-project-config --project-language typescript --output new.json"},
		{"guided authoring into a file in a directory says the directory must exist, because --output does not create it", Command{Mode: SuggestProjectConfig, Target: "cfg/new.json"}, "after making sure cfg exists at the repository root, run coach codesignal --baseline --suggest-project-config --project-language typescript --output cfg/new.json"},
		{"guided authoring into a path a shell would expand", Command{Mode: SuggestProjectConfig, Target: "$(touch PWNED).json"}, `coach codesignal --baseline --suggest-project-config --project-language typescript --output '$(touch PWNED).json'`},
		{"guided authoring for a named policy never names --project-config, which authoring rejects", Command{Mode: SuggestProjectConfig, Policy: policyref.Named("cfg/new.json")}, "coach codesignal --baseline --suggest-project-config --project-language typescript"},
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

func TestCommandPhraseNamesNoCommandForAnAuthoringTargetWhoseNameCannotBePrinted(t *testing.T) {
	const want = "run --suggest-project-config with --output naming the same path (its name is not printed here)"
	assertPhrase(t, Command{Mode: SuggestProjectConfig, Target: "bad\x1bname.json"}, "", want)
}

func TestBatchAuthoring(t *testing.T) {
	cases := map[string]struct{ target, want string }{
		"the default location":                {"", "coach codesignal --baseline --suggest-project-config"},
		"a file at the repository root":       {"new.json", "coach codesignal --baseline --suggest-project-config --output new.json"},
		"a file in a directory":               {"cfg/new.json", "after making sure cfg exists at the repository root, run coach codesignal --baseline --suggest-project-config --output cfg/new.json"},
		"a file in a nested directory":        {"a/b/new.json", "after making sure a/b exists at the repository root, run coach codesignal --baseline --suggest-project-config --output a/b/new.json"},
		"a path a shell would split":          {"my cfg/a b.json", "after making sure 'my cfg' exists at the repository root, run coach codesignal --baseline --suggest-project-config --output 'my cfg/a b.json'"},
		"a path whose name cannot be printed": {"bad\x1bname.json", "run --suggest-project-config with --output naming the same path (its name is not printed here)"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { assertBatchAuthoring(t, c.target, c.want) })
	}
}

func assertBatchAuthoring(t *testing.T, target, want string) {
	t.Helper()
	if got := BatchAuthoring(target); got != want {
		t.Errorf("BatchAuthoring(%q) = %q; want %q", target, got, want)
	}
}
