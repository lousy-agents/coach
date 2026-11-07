package projectconfig

import (
	"errors"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
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
// revision holds one whose document is not valid or is over the size budget; for a path with nothing at it the
// document is created, and a refusal that shares the kind of an invalid document offers neither.
func TestAnErrorReplacesThePolicyOnlyWhereItsDocumentIsNotValidAndANewOneFixesIt(t *testing.T) {
	cases := map[string]struct {
		err  *ConfigError
		want bool
	}{
		"invalid content that a new document replaces":                             {&ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyAuthor}, true},
		"a policy committed as a link":                                             {&ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone}, false},
		"a policy missing from disk":                                               {&ConfigError{Kind: KindMissing, Remedy: projectreadiness.RemedyAuthor}, false},
		"a policy found nowhere":                                                   {&ConfigError{Kind: KindNotFound, Remedy: projectreadiness.RemedyAuthor}, false},
		"a policy over the size budget, which a smaller document fixes":            {&ConfigError{Kind: KindUnreadable, Remedy: projectreadiness.RemedyAuthor}, true},
		"a policy whose read stopped on time or stderr, which no document changes": {&ConfigError{Kind: KindUnreadable, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseGitUnreadable}, false},
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

// The error a read that stopped on the size bound becomes is the one that replaces the policy; a read that stopped
// on the time bound is not, since no document changes why the read failed.
func TestTheReadOfAnOverBudgetCommittedPolicyReplacesItAndATimedOutReadDoesNot(t *testing.T) {
	cases := map[string]struct {
		kind gitrepo.BoundErrorKind
		want bool
	}{"over the budget": {gitrepo.BoundStdout, true}, "timed out": {gitrepo.BoundTimeout, false}}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheReadReplacesThePolicy(t, c.kind, c.want) })
	}
}

func requireTheReadReplacesThePolicy(t *testing.T, kind gitrepo.BoundErrorKind, want bool) {
	t.Helper()
	read := readError(nil, "repo", "HEAD", "cfg/p.json", &gitrepo.BoundError{Kind: kind})

	var configErr *ConfigError
	if !errors.As(read, &configErr) {
		t.Fatalf("readError = %v; want a *ConfigError", read)
	}
	requireReplacesPolicy(t, configErr, want)
}

// A new file can go at a path, or project.json be moved there, only where nothing is on disk and the judgment of the
// path arrived at a new policy as the way out. The index, a conflict, a mark, a sparse checkout, a submodule and the
// revision's own copy each leave another remedy, so none is free.
func TestAPathIsFreeForANewPolicyOnlyWhereNothingIsOnDiskAndAuthoringIsTheRemedy(t *testing.T) {
	cases := map[string]struct {
		err  *ConfigError
		want bool
	}{
		"nothing is there": {&ConfigError{Kind: KindMissing, Missing: MissingAbsent, Remedy: projectreadiness.RemedyAuthor}, true},
		"a staged copy that a new file would be added over": {&ConfigError{Kind: KindMissing, Missing: MissingAbsent, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseStagedCopy}, false},
		"a path inside a submodule":                         {&ConfigError{Kind: KindMissing, Missing: MissingAbsent, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseInSubmodule}, false},
		"a committed policy that a restore brings back":     {&ConfigError{Kind: KindMissing, Missing: MissingAbsent, Remedy: projectreadiness.RemedyRestore}, false},
		"a file where a directory should be":                {&ConfigError{Kind: KindMissing, Missing: MissingBlockedParent, Remedy: projectreadiness.RemedyAuthor}, false},
		"a path the disk cannot use":                        {&ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyAuthor}, false},
		"an error that was never classified":                {&ConfigError{}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireFreeForANewPolicy(t, c.err, c.want) })
	}
}

func requireFreeForANewPolicy(t *testing.T, err *ConfigError, want bool) {
	t.Helper()
	if got := err.FreeForANewPolicy(); got != want {
		t.Errorf("FreeForANewPolicy() = %v for kind %d, missing %d and remedy %d, want %v", got, err.Kind, err.Missing, err.Remedy, want)
	}
}

func TestOnlyAMissingPolicyIsAbsentFromDisk(t *testing.T) {
	cases := map[string]struct {
		err  *ConfigError
		want bool
	}{
		"nothing is there":         {&ConfigError{Kind: KindMissing, Missing: MissingAbsent}, true},
		"a file blocks the parent": {&ConfigError{Kind: KindMissing, Missing: MissingBlockedParent}, false},
		"an invalid document":      {&ConfigError{Kind: KindInvalid, Missing: MissingAbsent}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireAbsentFromDisk(t, c.err, c.want) })
	}
}

func requireAbsentFromDisk(t *testing.T, err *ConfigError, want bool) {
	t.Helper()
	if got := err.AbsentFromDisk(); got != want {
		t.Errorf("AbsentFromDisk() = %v, want %v", got, want)
	}
}
