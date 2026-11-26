package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// projectLanguageFlag is --project-language as the command line gave it. A value alone cannot say whether the flag
// was given, because go is both its default and a language it may name.
type projectLanguageFlag struct {
	value string
	given bool
}

// chosenProjectLanguage is the language a project analysis runs in: the flag's if it was given, otherwise the project
// policy's, otherwise go.
type chosenProjectLanguage struct {
	scope codesignal.ProjectLanguageScope
	// overridden is the language the policy names where the flag names another, and empty otherwise.
	overridden projectconfig.Language
}

func chooseProjectLanguage(flag projectLanguageFlag, policy projectconfig.Language) chosenProjectLanguage {
	switch {
	case flag.given:
		chosen := chosenProjectLanguage{scope: codesignal.ProjectLanguageScope{Language: flag.value, Source: codesignal.ProjectLanguageSourceFlag}}
		if policy != "" && string(policy) != flag.value {
			chosen.overridden = policy
		}
		return chosen
	case policy != "":
		return chosenProjectLanguage{scope: codesignal.ProjectLanguageScope{Language: string(policy), Source: codesignal.ProjectLanguageSourcePolicy}}
	}
	return chosenProjectLanguage{scope: codesignal.ProjectLanguageScope{Language: string(projectconfig.LanguageGo), Source: codesignal.ProjectLanguageSourceDefault}}
}

// overrideDiagnostic is the diagnostic for a flag that disagrees with the policy, and whether there is one.
func (c chosenProjectLanguage) overrideDiagnostic(policyPath string) (codesignal.Diagnostic, bool) {
	if c.overridden == "" {
		return codesignal.Diagnostic{}, false
	}
	return codesignal.Diagnostic{
		Kind:    codesignal.DiagKindProjectLanguageOverridesPolicy,
		Path:    policyPath,
		Message: fmt.Sprintf("--project-language %s was used; the project policy names %s", c.scope.Language, c.overridden),
	}, true
}
