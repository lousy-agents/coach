package main

import (
	"errors"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

// authoringOffer is how a new policy reaches the user for an error: not at all, as a guided session that runs
// now, or as a command to run on a terminal.
type authoringOffer int

const (
	authoringNotOffered authoringOffer = iota
	authoringSession
	authoringCommand
)

// offerAuthoring is the one decision about authoring. Whether a new policy is offered is the remedy the error
// names (OffersAuthoring), for a policy named with --project-config and for a discovered one alike. How it is
// offered follows from that and from who can answer: a TypeScript scan with a person to answer opens the
// session, and anything else gets the command to run on a terminal. A gate that opens the session and the line
// that prints the command both ask this, so a terminal cannot remove the only way out.
func offerAuthoring(err error, language string, interactive bool) authoringOffer {
	var configErr *projectconfig.ConfigError
	if !errors.As(err, &configErr) || !configErr.OffersAuthoring() {
		return authoringNotOffered
	}
	if language == "typescript" && interactive {
		return authoringSession
	}
	return authoringCommand
}

// authoringInteractive is whether a person can answer a guided session: a controlling terminal, and no request
// for an unattended run.
func authoringInteractive(noInteractive bool) bool {
	return !noInteractive && terminal.HasControllingTerminal(os.Stdin)
}

// authoringRemediation is the appended line that offers a new policy as a command to run on a terminal, and
// nothing when authoring cannot fix the error (the policy is committed and only needs restoring, or its path cannot
// be created) or when a guided session runs instead. It writes the policy the user named when nothing is at that
// path, so it names that path; for a document that exists and cannot be used it names the scan, which can replace it, or
// for Go the candidate that is written beside it and the move that replaces it.
func authoringRemediation(configErr *projectconfig.ConfigError, policy policyref.Ref, language string, interactive bool, dir string) string {
	if offerAuthoring(configErr, language, interactive) != authoringCommand {
		return ""
	}
	switch {
	case configErr.ReplacesPolicy() && language == "typescript":
		return tssetup.ReplacePolicyRemediation(policy)
	case configErr.ReplacesPolicy():
		return goReplaceRemediation(dir, policy)
	}
	return tssetup.SuggestProjectConfigRemediation(language, authoringTargetPolicy(configErr, policy))
}

// authoringTargetPolicy is the policy authoring should write: the one the user
// named when nothing is at its path, and none otherwise. Authoring is
// create-only, so a policy that exists and is invalid cannot be its --output
// target; the command is offered with authoring's default location, as before.
func authoringTargetPolicy(configErr *projectconfig.ConfigError, policy policyref.Ref) policyref.Ref {
	if configErr.Kind == projectconfig.KindMissing || configErr.Kind == projectconfig.KindNotFound {
		return policy
	}
	return policyref.Ref{}
}
