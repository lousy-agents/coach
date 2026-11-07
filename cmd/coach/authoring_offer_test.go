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
// otherwise. The two cannot disagree, so a terminal never removes the only way out.
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
		"a committed policy whose document is not valid, on a terminal":    {configError(projectconfig.KindInvalid, author), "typescript", true, authoringSession},
		"a committed policy whose document is not valid, nobody to answer": {configError(projectconfig.KindInvalid, author), "typescript", false, authoringCommand},
		"the same, wrapped": {fmt.Errorf("wrapped: %w", configError(projectconfig.KindInvalid, author)), "typescript", true, authoringSession},
		"a Go scan has no session, whatever the terminal": {configError(projectconfig.KindMissing, author), "go", true, authoringCommand},
		"a policy committed as a link":                    {&projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseCommittedAsLink}, "typescript", true, authoringNotOffered},
		"a policy committed as a directory":               {&projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseDirectory}, "typescript", true, authoringNotOffered},
		"a restore over an empty index entry":             {configError(projectconfig.KindMissing, projectreadiness.RemedyRestoreOverEmptyEntry), "typescript", true, authoringNotOffered},
		"a policy a restore fixes":                        {configError(projectconfig.KindMissing, projectreadiness.RemedyRestore), "typescript", true, authoringNotOffered},
		"a policy no remedy fixes":                        {configError(projectconfig.KindInvalid, projectreadiness.RemedyNone), "typescript", true, authoringNotOffered},
		"an error whose remedy nobody set":                {configError(projectconfig.KindMissing, 0), "typescript", true, authoringNotOffered},
		"an error that is not about a policy":             {errors.New("boom"), "typescript", true, authoringNotOffered},
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

// A document that exists and is not valid is replaced, not created: the closing instruction of a session says to
// save the candidate over the file, and one for a path with nothing at it says to save it there.
func TestTheClosingInstructionSaysWhereTheCandidateGoes(t *testing.T) {
	cases := map[string]struct {
		err    error
		flags  codesignalFlags
		expect string
	}{
		"a named policy whose document is not valid":      {configError(projectconfig.KindInvalid, projectreadiness.RemedyAuthor), namedPolicyFlags("pol.json"), `save it as "pol.json" replacing the current file`},
		"a discovered policy whose document is not valid": {configError(projectconfig.KindInvalid, projectreadiness.RemedyAuthor), codesignalFlags{}, `save it as ".lousy-agents/coach.json" replacing the current file`},
		"a named policy missing from disk":                {configError(projectconfig.KindMissing, projectreadiness.RemedyAuthor), namedPolicyFlags("pol.json"), `save it to "pol.json" at the repository root`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheClosingInstruction(t, c.err, c.flags, c.expect) })
	}
}

func requireTheClosingInstruction(t *testing.T, err error, flags codesignalFlags, expect string) {
	t.Helper()
	if got := candidateClosingInstruction(err, flags); !strings.Contains(got, expect) {
		t.Errorf("candidateClosingInstruction = %q; want it to say %q", got, expect)
	}
}

func namedPolicyFlags(path string) codesignalFlags {
	return codesignalFlags{namedPolicy: policyref.Named(path)}
}

// The command printed for a document that exists and is not valid is the scan, which on a terminal opens the
// session that can replace it. Authoring creates files and refuses a target that exists, so the authoring command
// would leave the invalid file as it was; for a path with nothing at it that command is the one that works.
func TestThePrintedCommandCanReplaceTheFileItIsPrintedFor(t *testing.T) {
	invalid := configError(projectconfig.KindInvalid, projectreadiness.RemedyAuthor)
	missing := configError(projectconfig.KindMissing, projectreadiness.RemedyAuthor)

	replace := authoringRemediation(invalid, policyref.Named("pol.json"), "typescript", false)
	create := authoringRemediation(missing, policyref.Named("pol.json"), "typescript", false)
	discovered := authoringRemediation(invalid, policyref.Ref{}, "typescript", false)

	if !strings.Contains(replace, "coach codesignal --baseline --project-config pol.json --project-language typescript") || strings.Contains(replace, "--suggest-project-config") {
		t.Errorf("remediation for an invalid named policy = %q; want the scan, not authoring", replace)
	}
	if !strings.Contains(discovered, "coach codesignal --baseline --project-language typescript") || strings.Contains(discovered, "--suggest-project-config") || strings.Contains(discovered, "--project-config") {
		t.Errorf("remediation for an invalid discovered policy = %q; want the scan without --project-config, and not authoring", discovered)
	}
	if !strings.Contains(create, "--suggest-project-config") || !strings.Contains(create, "--output pol.json") {
		t.Errorf("remediation for a missing named policy = %q; want authoring into the named path", create)
	}
}

func TestNoCommandIsPrintedWhereASessionRunsOrNothingIsOffered(t *testing.T) {
	invalid := configError(projectconfig.KindInvalid, projectreadiness.RemedyAuthor)
	restore := configError(projectconfig.KindMissing, projectreadiness.RemedyRestore)

	if got := authoringRemediation(invalid, policyref.Named("pol.json"), "typescript", true); got != "" {
		t.Errorf("remediation while a session runs = %q; want none", got)
	}
	if got := authoringRemediation(restore, policyref.Named("pol.json"), "typescript", false); got != "" {
		t.Errorf("remediation for a policy a restore fixes = %q; want none", got)
	}
}
