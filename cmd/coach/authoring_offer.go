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
//
// The exception is a TypeScript policy that exists and cannot be used: the only session TypeScript has ends by asking
// the user to save a candidate over that file, since a scan cannot pass --output, so it is offered neither as a
// session nor as a line. Go has no session and gets a candidate written beside the policy.
func offerAuthoring(err error, language string, interactive bool) authoringOffer {
	var configErr *projectconfig.ConfigError
	if !errors.As(err, &configErr) || !configErr.OffersAuthoring() {
		return authoringNotOffered
	}
	if language == "typescript" && configErr.ReplacesPolicy() {
		return authoringNotOffered
	}
	if language == "typescript" && interactive {
		return authoringSession
	}
	return authoringCommand
}

// interactive is whether a person can answer a guided session: a controlling terminal, and no request for an
// unattended run. It is pure, so that the decision is pinned wherever the tests run.
func interactive(noInteractive, hasTerminal bool) bool {
	return !noInteractive && hasTerminal
}

// authoringInteractive is interactive for the terminal this process is attached to.
func authoringInteractive(noInteractive bool) bool {
	return interactive(noInteractive, terminal.HasControllingTerminal(os.Stdin))
}

// authoringRemediation is the appended line that offers a new policy as a command to run on a terminal, and
// nothing when authoring cannot fix the error (the policy is committed and only needs restoring, or its path cannot
// be created) or when a guided session runs instead. It writes the policy the user named when nothing is at that
// path, so it names that path. For a document that exists and cannot be used, Go gets the candidate that is written
// beside it, with words that say to replace the policy with it; TypeScript is not offered authoring at all
// (offerAuthoring), so it gets the words that say how to get past the policy (unusablePolicyStep), terminal or not.
func authoringRemediation(configErr *projectconfig.ConfigError, policy policyref.Ref, language string, interactive bool, dir string) string {
	if language == "typescript" && configErr.ReplacesPolicy() {
		return "coach codesignal: " + unusablePolicyStep + "."
	}
	if offerAuthoring(configErr, language, interactive) != authoringCommand {
		return ""
	}
	if configErr.ReplacesPolicy() {
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
