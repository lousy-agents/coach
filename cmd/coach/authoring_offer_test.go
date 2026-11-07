package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// Whether a new policy is offered is decided once, from the remedy the error names. How it reaches the user is
// chosen afterwards: a guided session when a person can answer a TypeScript scan, a command to run on a terminal
// otherwise. The two cannot disagree, so a terminal never removes the only way out. A document that exists and cannot
// be used is the exception for TypeScript: its only session ends by saving a replacement over the user's file, so it is
// offered neither as a session nor as a line.
func TestAuthoringIsOfferedWhereTheRemedyIsANewPolicy(t *testing.T) {
	author := projectreadiness.RemedyAuthor
	cases := map[string]struct {
		err         error
		language    string
		interactive bool
		want        authoringOffer
	}{
		"a missing policy, on a terminal":                                  {configError(projectconfig.KindMissing, author), "typescript", true, authoringSession},
		"a missing policy, with nobody to answer":                          {configError(projectconfig.KindMissing, author), "typescript", false, authoringCommand},
		"a policy found nowhere, on a terminal":                            {configError(projectconfig.KindNotFound, author), "typescript", true, authoringSession},
		"a committed policy whose document is not valid, on a terminal":    {configError(projectconfig.KindInvalid, author), "typescript", true, authoringNotOffered},
		"a committed policy whose document is not valid, nobody to answer": {configError(projectconfig.KindInvalid, author), "typescript", false, authoringNotOffered},
		"the same, wrapped": {fmt.Errorf("wrapped: %w", configError(projectconfig.KindInvalid, author)), "typescript", true, authoringNotOffered},
		"a committed policy whose document is not valid, in a Go scan": {configError(projectconfig.KindInvalid, author), "go", true, authoringCommand},
		"a Go scan has no session, whatever the terminal":              {configError(projectconfig.KindMissing, author), "go", true, authoringCommand},
		"a policy committed as a link":                                 {&projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseCommittedAsLink}, "typescript", true, authoringNotOffered},
		"a policy committed as a directory":                            {&projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseDirectory}, "typescript", true, authoringNotOffered},
		"a restore over an empty index entry":                          {configError(projectconfig.KindMissing, projectreadiness.RemedyRestoreOverEmptyEntry), "typescript", true, authoringNotOffered},
		"a policy a restore fixes":                                     {configError(projectconfig.KindMissing, projectreadiness.RemedyRestore), "typescript", true, authoringNotOffered},
		"a policy no remedy fixes":                                     {configError(projectconfig.KindInvalid, projectreadiness.RemedyNone), "typescript", true, authoringNotOffered},
		"an error whose remedy nobody set":                             {configError(projectconfig.KindMissing, 0), "typescript", true, authoringNotOffered},
		"an error that is not about a policy":                          {errors.New("boom"), "typescript", true, authoringNotOffered},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireAuthoringOffer(t, c.err, c.language, c.interactive, c.want) })
	}
}

func configError(kind projectconfig.ErrorKind, remedy projectreadiness.PolicyRemedy) *projectconfig.ConfigError {
	return &projectconfig.ConfigError{Kind: kind, Remedy: remedy}
}

func requireAuthoringOffer(t *testing.T, err error, language string, interactive bool, want authoringOffer) {
	t.Helper()
	if got := offerAuthoring(err, language, interactive); got != want {
		t.Errorf("offerAuthoring(%v, %q, interactive %v) = %v, want %v", err, language, interactive, got, want)
	}
}

// The closing instruction of a session says where the candidate goes: at the path the user named, or at the default
// path, when nothing is there yet. A session is never opened for a document that exists and cannot be used, so the
// instruction never says to save a candidate over a file.
func TestTheClosingInstructionSaysWhereTheCandidateGoes(t *testing.T) {
	cases := map[string]struct {
		flags  codesignalFlags
		expect string
	}{
		"a named policy":     {namedPolicyFlags("pol.json"), `save it to "pol.json" at the repository root`},
		"the default policy": {codesignalFlags{}, `save it to ".lousy-agents/coach.json" at the repository root`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheClosingInstruction(t, c.flags, c.expect) })
	}
}

func requireTheClosingInstruction(t *testing.T, flags codesignalFlags, expect string) {
	t.Helper()
	if got := candidateClosingInstruction(flags); !strings.Contains(got, expect) {
		t.Errorf("candidateClosingInstruction = %q; want it to say %q", got, expect)
	}
}

func namedPolicyFlags(path string) codesignalFlags {
	return codesignalFlags{namedPolicy: policyref.Named(path)}
}

// Authoring creates files and refuses a target that exists. For a path with nothing at it the line is the authoring
// command that works. For a TypeScript document that exists and cannot be used there is no command and no session, because
// the only session it could name ends by saving a replacement over the user's file; the line says in words to fix the
// policy, or to move it aside and commit the move and then run guided policy authoring. That sentence is the one
// --prepare-compiler prints, from one source.
func TestTypeScriptSaysInWordsHowToGetPastADocumentThatExistsAndNamesTheAuthoringCommandForOneThatDoesNot(t *testing.T) {
	invalid := configError(projectconfig.KindInvalid, projectreadiness.RemedyAuthor)
	missing := configError(projectconfig.KindMissing, projectreadiness.RemedyAuthor)

	replace := authoringRemediation(invalid, policyref.Named("pol.json"), "typescript", false, "")
	onATerminal := authoringRemediation(invalid, policyref.Named("pol.json"), "typescript", true, "")
	discovered := authoringRemediation(invalid, policyref.Ref{}, "typescript", false, "")
	create := authoringRemediation(missing, policyref.Named("pol.json"), "typescript", false, "")

	const want = "coach codesignal: the committed policy cannot be used: fix it, or move it aside and commit the move, then run guided policy authoring to write a new one."
	for name, got := range map[string]string{"an invalid named policy": replace, "the same on a terminal": onATerminal, "an invalid discovered policy": discovered} {
		if got != want {
			t.Errorf("remediation for %s = %q; want %q", name, got, want)
		}
	}
	if !strings.Contains(create, "--suggest-project-config") || !strings.Contains(create, "--output pol.json") {
		t.Errorf("remediation for a missing named policy = %q; want authoring into the named path", create)
	}
}

func TestNoCommandIsPrintedWhereASessionRunsOrNothingIsOffered(t *testing.T) {
	restore := configError(projectconfig.KindMissing, projectreadiness.RemedyRestore)
	missing := configError(projectconfig.KindMissing, projectreadiness.RemedyAuthor)
	if got := authoringRemediation(missing, policyref.Named("pol.json"), "typescript", true, ""); got != "" {
		t.Errorf("remediation while a session runs = %q; want none", got)
	}
	if got := authoringRemediation(restore, policyref.Named("pol.json"), "typescript", false, ""); got != "" {
		t.Errorf("remediation for a policy a restore fixes = %q; want none", got)
	}
}
