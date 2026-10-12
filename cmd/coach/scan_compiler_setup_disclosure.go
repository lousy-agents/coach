package main

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

// withheldSetupChoicesLine renders AvailableSetupChoices' own reasons for
// ruling out every candidate, for the one outcome that leaves the customer
// without a next step: a real compiler gap where nothing at all could be
// offered. The gap line printed above it names --check-project, which only
// re-reports the same gap, so without the reasons (an unconfigured mise, a
// manifest that declares no supported compiler, a rejected package manager)
// there is nothing to act on. It returns "" when no menu was built, so the
// runtime-boundary path's output is unchanged.
func withheldSetupChoicesLine(withheld []tssetup.WithheldChoice) string {
	if len(withheld) == 0 {
		return ""
	}
	reasons := make([]string, 0, len(withheld))
	for _, entry := range withheld {
		reasons = append(reasons, fmt.Sprintf("%s (%s)", entry.Kind, entry.Reason))
	}
	return "coach codesignal: no compiler-setup choice is executable here: " + strings.Join(reasons, ", ") + "."
}

// setupResidueDisclosure renders AC-SET-7's "identify files that may have
// changed" for a failed setup. ResidueUnknown is not a quieter version of an
// empty ChangedPaths: it means Coach could not read what changed at all, and
// the fallback path it carries is the working directory itself -- which
// renders repository-relative as a bare ".", indistinguishable from a precise
// finding. Saying so plainly is the difference between "nothing changed" and
// "Coach does not know", which is exactly the distinction SetupOutcome's own
// contract asks callers to preserve.
func setupResidueDisclosure(result tssetup.CompilerSetupOfferResult) string {
	if result.ResidueUnknown {
		if len(result.ChangedPaths) > 0 {
			return "coach codesignal: Coach could not determine which files the setup command changed under " + strings.Join(result.ChangedPaths, ", ") + "; inspect it before rerunning."
		}
		return "coach codesignal: Coach could not determine which files the setup command changed; inspect the working tree before rerunning."
	}
	if len(result.ChangedPaths) == 0 {
		return ""
	}
	return "coach codesignal: the setup command may have changed: " + strings.Join(result.ChangedPaths, ", ")
}
