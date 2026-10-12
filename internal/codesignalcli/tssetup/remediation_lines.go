package tssetup

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func alsoFailingGapLine(code, configPath string) string {
	return code + ": also failing, run " + typescriptInvocation("--check-project", configPath)
}

// AlsoFailingGapLines is AC-SET-13's "report all gaps" clause: one line for
// every gap readiness itself reports other than the policy failure the scan
// has already printed as its own message.
//
// It is reached only alongside that policy failure
// (ProjectConfigErrorWithReadiness is the sole production producer). Before
// R1, a missing policy left checkProjectShape and pkgmanager.Check
// guessing from the worktree root in place of the roots a policy would have
// selected, so a gap either of them raised might simply be an artifact of
// that guess -- which is why this line used to hedge rather than assert the
// gap would still be there. Both checks now report not_checked instead of
// guessing (R1), so every gap readiness.Gaps still carries here -- the
// compiler check, the runtime check, and a mise-scope trust finding -- was
// never roots-dependent in the first place, real and independent of the
// policy either way. So this simply reports readiness.Gaps unchanged, with
// no hedge left to state.
//
// readiness.Gaps is already emitted in the epic's frozen next-action order,
// so iterating it preserves that ordering rather than inventing one here.
func AlsoFailingGapLines(readiness *projectreadiness.Result, configPath string) []string {
	if readiness == nil {
		return nil
	}
	var lines []string
	seen := make(map[string]bool, len(readiness.Gaps))
	for _, gap := range readiness.Gaps {
		if gap.Code == projectreadiness.GapPolicyMissing || gap.Code == projectreadiness.GapPolicyInvalid || seen[gap.Code] {
			continue
		}
		seen[gap.Code] = true
		lines = append(lines, alsoFailingGapLine(gap.Code, configPath))
	}
	return lines
}

// SuggestProjectConfigRemediation names the --suggest-project-config
// invocation that resolves a projectconfig.ConfigError gap for language, for
// AC-SET-9's appended no-controlling-terminal remediation line. For
// "typescript" this is the interactive, guided policy-authoring command; for
// every other language (only "go" reaches this today) it is the plain batch
// candidate-generation command, since --project-language typescript would
// name a command that language cannot run (AC-2: an offered remediation must
// actually be supported for the language it is offered to). It never appends
// a --project-config suffix: validateSuggestProjectConfigFlags rejects
// --suggest-project-config combined with --project-config.
//
// The TypeScript form qualifies the command rather than naming it bare,
// because this line is printed precisely when no controlling terminal is
// available and guided authoring refuses without one: an agent or CI job
// that runs it verbatim gets exit 2 and no policy. The refusal's own
// instruction -- draft the document, have it reviewed and committed -- is
// the path actually open here, so it is stated alongside the command rather
// than discovered by spending an invocation on it.
func SuggestProjectConfigRemediation(language string) string {
	if language == "typescript" {
		return onATerminal(typescriptInvocation("--suggest-project-config", "")) +
			" -- guided authoring requires a controlling terminal; without one, draft the schema-1 project-config document yourself, have a human review and commit it, then rerun with --project-config <path>"
	}
	return "coach codesignal --baseline --suggest-project-config"
}

// AppendedRemediationLine withholds line whenever hasControllingTerminal is
// true and language is "typescript": the interactive setup offer itself owns
// that case there, so AC-SET-9's appended command is printed only when no
// controlling terminal is available to run it. No such offer exists for any
// other language (only "go" reaches this today), so a controlling-terminal
// user must still see the same appended remediation a piped invocation gets.
func AppendedRemediationLine(hasControllingTerminal bool, language, line string) string {
	if hasControllingTerminal && language == "typescript" {
		return ""
	}
	return line
}
