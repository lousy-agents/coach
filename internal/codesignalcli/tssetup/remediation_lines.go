package tssetup

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/invocation"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func alsoFailingGapLine(code string, policy policyref.Ref, rerun bool) string {
	if !rerun {
		return code + ": also failing"
	}
	return code + ": also failing, " + invocation.Command{Mode: invocation.CheckProject, Policy: policy}.Phrase("run")
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
func AlsoFailingGapLines(readiness *projectreadiness.Result, policy policyref.Ref) []string {
	return alsoFailingGapLines(readiness, policy, true)
}

// alsoFailingGapLines is AlsoFailingGapLines, with the rerun command left off each line when rerun is false.
func alsoFailingGapLines(readiness *projectreadiness.Result, policy policyref.Ref, rerun bool) []string {
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
		lines = append(lines, alsoFailingGapLine(gap.Code, policy, rerun))
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
// --suggest-project-config combined with --project-config. A policy the user
// named somewhere authoring does not write by default is named with --output
// instead, because the command is offered to make that path exist: without it
// the command would succeed, write the default location, and leave the named
// path as missing as before.
//
// The TypeScript form qualifies the command rather than naming it bare,
// because this line is printed precisely when no controlling terminal is
// available and guided authoring refuses without one: an agent or CI job
// that runs it verbatim gets exit 2 and no policy. The refusal's own
// instruction -- draft the document, have it reviewed and committed -- is
// the path actually open here, so it is stated alongside the command rather
// than discovered by spending an invocation on it.
func SuggestProjectConfigRemediation(language string, policy policyref.Ref) string {
	target := authoringTarget(policy)
	if language == "typescript" {
		return onATerminal(invocation.Command{Mode: invocation.SuggestProjectConfig, Target: target}.Phrase("")) +
			" -- guided authoring requires a controlling terminal; without one, draft the schema-1 project-config document yourself, have a human review and commit it, then rerun with --project-config <path>"
	}
	return invocation.BatchAuthoring(target)
}

// ReplacePolicyRemediation is the appended line for a policy that exists and whose document is not valid. The
// authoring command creates files and refuses a target that exists, and written elsewhere its candidate would
// leave this file as it was, so the command is the scan itself: on a terminal it opens guided authoring and prints
// a replacement to save over the file. Like every appended authoring command it needs a terminal, which the
// prefix says.
func ReplacePolicyRemediation(policy policyref.Ref) string {
	return onATerminal(invocation.Command{Mode: invocation.Scan, Policy: policy}.Phrase("")) +
		" -- guided authoring opens there and prints a replacement for the invalid document; save it over the file, have a human review and commit it, then rerun"
}

// authoringTarget is where authoring has to write for the policy the user named
// to exist: empty when nothing was named, or when the name is where authoring
// writes by default.
func authoringTarget(policy policyref.Ref) string {
	named, ok := policy.NamedPath()
	if !ok || named == projectconfig.CanonicalPath {
		return ""
	}
	return named
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
