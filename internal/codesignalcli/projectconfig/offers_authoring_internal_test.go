package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// Starting or suggesting guided authoring is one decision, and the remedy makes it: a new policy is
// offered exactly when the remedy is a new policy. The kind of error says what went wrong, and a
// gate that asks it instead offers authoring for every refusal that merely shares the kind.
func TestAnErrorOffersAuthoringExactlyWhenItsRemedyIsANewPolicy(t *testing.T) {
	cases := map[string]struct {
		err  *ConfigError
		want bool
	}{
		"invalid content that a new document replaces": {&ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyAuthor}, true},
		"a policy missing from disk":                   {&ConfigError{Kind: KindMissing, Remedy: projectreadiness.RemedyAuthor}, true},
		"a policy committed as a link":                 {&ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone}, false},
		"a policy the working tree cannot use":         {&ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseSymbolicLink}, false},
		"a policy that is restored":                    {&ConfigError{Kind: KindMissing, Remedy: projectreadiness.RemedyRestore}, false},
		"a restore over an empty entry":                {&ConfigError{Kind: KindMissing, Remedy: projectreadiness.RemedyRestoreOverEmptyEntry}, false},
		"an error that was never classified":           {&ConfigError{}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireOffersAuthoring(t, c.err, c.want) })
	}
}

func requireOffersAuthoring(t *testing.T, err *ConfigError, want bool) {
	t.Helper()
	if got := err.OffersAuthoring(); got != want {
		t.Errorf("OffersAuthoring() = %v for kind %d and remedy %d, want %v", got, err.Kind, err.Remedy, want)
	}
}

// Authoring creates files and refuses a target that exists, so a new document replaces a policy only when the
// revision holds one whose document is not valid; for a path with nothing at it the document is created, and a
// refusal that shares the kind of an invalid document offers neither.
func TestAnErrorReplacesThePolicyOnlyWhereItsDocumentIsNotValidAndANewOneFixesIt(t *testing.T) {
	cases := map[string]struct {
		err  *ConfigError
		want bool
	}{
		"invalid content that a new document replaces": {&ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyAuthor}, true},
		"a policy committed as a link":                 {&ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone}, false},
		"a policy missing from disk":                   {&ConfigError{Kind: KindMissing, Remedy: projectreadiness.RemedyAuthor}, false},
		"a policy found nowhere":                       {&ConfigError{Kind: KindNotFound, Remedy: projectreadiness.RemedyAuthor}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireReplacesPolicy(t, c.err, c.want) })
	}
}

func requireReplacesPolicy(t *testing.T, err *ConfigError, want bool) {
	t.Helper()
	if got := err.ReplacesPolicy(); got != want {
		t.Errorf("ReplacesPolicy() = %v for kind %d and remedy %d, want %v", got, err.Kind, err.Remedy, want)
	}
}
