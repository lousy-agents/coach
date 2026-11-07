package main

import (
	"errors"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"
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
