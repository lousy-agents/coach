package main

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestChooseProjectLanguage(t *testing.T) {
	cases := []struct {
		name       string
		flag       projectLanguageFlag
		policy     projectconfig.Language
		want       codesignal.ProjectLanguageScope
		overridden projectconfig.Language
	}{
		{"neither names a language", projectLanguageFlag{value: "go"}, "", codesignal.ProjectLanguageScope{Language: "go", Source: "default"}, ""},
		{"only the policy names one", projectLanguageFlag{value: "go"}, "typescript", codesignal.ProjectLanguageScope{Language: "typescript", Source: "policy"}, ""},
		{"only the flag names one", projectLanguageFlag{value: "typescript", given: true}, "", codesignal.ProjectLanguageScope{Language: "typescript", Source: "flag"}, ""},
		{"the flag agrees with the policy", projectLanguageFlag{value: "go", given: true}, "go", codesignal.ProjectLanguageScope{Language: "go", Source: "flag"}, ""},
		{"the flag disagrees with the policy", projectLanguageFlag{value: "go", given: true}, "typescript", codesignal.ProjectLanguageScope{Language: "go", Source: "flag"}, "typescript"},
		{"a flag value that is only the default does not override the policy", projectLanguageFlag{value: "go"}, "typescript", codesignal.ProjectLanguageScope{Language: "typescript", Source: "policy"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { expectChosenLanguage(t, c.flag, c.policy, c.want, c.overridden) })
	}
}

func expectChosenLanguage(t *testing.T, flag projectLanguageFlag, policy projectconfig.Language, want codesignal.ProjectLanguageScope, overridden projectconfig.Language) {
	t.Helper()
	got := chooseProjectLanguage(flag, policy)
	if got.scope != want || got.overridden != overridden {
		t.Errorf("got %+v (overridden %q), want %+v (overridden %q)", got.scope, got.overridden, want, overridden)
	}
	_, overrides := got.overrideDiagnostic(".lousy-agents/coach.json")
	if overrides != (overridden != "") {
		t.Errorf("a diagnostic for the override: got %v, want %v", overrides, overridden != "")
	}
}
