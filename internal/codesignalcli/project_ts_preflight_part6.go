package codesignalcli

import (
	"fmt"
)

// typescriptScanInvocation names the bare `coach codesignal --baseline`
// scan itself, distinct from typescriptInvocation's own --check-project/
// --prepare-compiler forms: R2's remediation points at rerunning the
// original scan on a terminal, not at a standalone subcommand.
func typescriptScanInvocation(configPath string) string {
	invocation := "coach codesignal --baseline"
	if configPath != "" {
		invocation += " --project-config " + configPath
	}
	return invocation + " --project-language typescript"
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

// setupChoiceScopeClause names what each choice would touch, at the moment
// the customer picks one. The full AC-SET-2 preview still precedes the
// confirmation, but it arrives only after a selection, so without this the
// selection itself is made from bare machine identifiers -- and
// project_mise and global_mise differ in exactly the property a customer
// would want to know before choosing between them.
func setupChoiceScopeClause(kind SetupChoiceKind) string {
	switch kind {
	case SetupChoiceProjectPackage:
		return " (runs this project's own package manager in the selected manifest context)"
	case SetupChoiceProjectMise:
		return " (installs the version this repository's mise configuration pins, into mise's shared tool store)"
	case SetupChoiceGlobalMise:
		return " (installs the version your global mise configuration pins, into mise's shared tool store)"
	case SetupChoiceCancel:
		return " (change nothing and stop this scan)"
	default:
		return ""
	}
}

func typescriptInvocation(flag, projectConfigPath string) string {
	invocation := "coach codesignal --baseline " + flag + " --project-language typescript"
	if projectConfigPath != "" {
		invocation += " --project-config " + projectConfigPath
	}
	return invocation
}

func miseOriginForSetupChoiceKind(kind SetupChoiceKind) string {
	if kind == SetupChoiceGlobalMise {
		return compilerOriginMiseGlobal
	}
	return compilerOriginMiseProject
}

func miseSetupOfferFailureDetail(installed miseInstallResult, version, origin string) string {
	switch {
	case installed.Observed && installed.Class != "":
		return fmt.Sprintf("mise install exited 0 but the installed TypeScript %s is not eligible (%s)", version, installed.Class)
	case installed.Attempted:
		return fmt.Sprintf("mise install failed for TypeScript %s under the %s scope", version, origin)
	case installed.Code != "":
		return fmt.Sprintf("mise install could not even be started (%s)", installed.Code)
	default:
		return "mise install could not even be started"
	}
}
