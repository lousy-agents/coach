// Command coach is the composition-root CLI for the coach project.
package main

import (
	"bytes"

	"flag"
	"fmt"
	"io"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// version is overridden via -ldflags at release; a local build reports "dev".
var version = "dev"

var (
	loadProjectConfig     = codesignalcli.LoadProjectConfig
	resolveProjectBackend = codesignalcli.ResolveProjectBackend
	lookupProjectBackend  = func(language string) codesignalcli.ProjectBackend {
		switch language {
		case "go":
			return codesignalcli.NewGoProjectBackend()
		case "typescript":
			return codesignalcli.NewTSProjectBackend()
		default:
			return nil
		}
	}
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const topLevelUsage = `usage: coach <command> [flags]

commands:
  codesignal   analyze production-code readiness signals in a Git diff or baseline

run "coach codesignal --help" for command-specific help.`

var codesignalUsage = "usage: coach codesignal (--base <ref> | --baseline) [--format text|json] [--scope production|all] [--min-severity " + severityFloorsUsage() + "] [--top N] [--build-target <package>] [--project-config <path>] [--project-language go|typescript] [--no-interactive] [--fail-on-incomplete-coverage]\n   or: coach codesignal --baseline --suggest-project-config [--output <path>]\n   or: coach codesignal --baseline --suggest-project-config --project-language typescript [--output <path>] [--no-interactive]\n   or: coach codesignal --baseline --check-project --project-language typescript [--project-config <path>] [--format text|json]\n   or: coach codesignal --baseline --prepare-compiler --project-language typescript [--project-config <path>] [--no-interactive]"

type codesignalFlags struct {
	base                     string
	baseline                 bool
	format                   string
	scope                    string
	buildTarget              string
	projectConfig            string
	projectLanguage          string
	minSeverity              string
	minSeveritySet           bool
	top                      string
	topSet                   bool
	projectConfigSet         bool
	suggestProjectConfig     bool
	output                   string
	outputSet                bool
	checkProject             bool
	prepareCompiler          bool
	noInteractive            bool
	failOnIncompleteCoverage bool

	// args is the raw argument list the flags were parsed from, kept so a
	// report can name the command that re-runs the same invocation.
	args []string
}

// countingBoolFlag is a flag.Value wrapper that counts how many times Set
// was called, so parseCodesignalFlags can detect a flag supplied more than
// once (flag.FlagSet's normal Bool/String accessors silently keep only the
// last value).
type countingBoolFlag struct {
	value bool
	count int
}

func (c *countingBoolFlag) IsBoolFlag() bool { return true }

type countingStringFlag struct {
	value string
	count int
}

func (c *countingStringFlag) Set(s string) error {
	c.value = s
	c.count++
	return nil
}

func writeSuggestInvalidArguments(stderr *os.File, message string) {
	stderr.Write(codesignalcli.InvalidArgumentsSuggestionEnvelope(message))
}

type codesignalFlagHolders struct {
	base                     *string
	baseline                 *bool
	format                   *string
	scope                    *string
	buildTarget              *string
	projectConfig            *string
	projectLanguage          *string
	minSeverity              *string
	top                      *string
	suggestProjectConfig     *countingBoolFlag
	output                   *countingStringFlag
	checkProject             *countingBoolFlag
	prepareCompiler          *countingBoolFlag
	noInteractive            *bool
	failOnIncompleteCoverage *bool
}

func registerCodesignalFlags(flags *flag.FlagSet) codesignalFlagHolders {
	h := codesignalFlagHolders{
		base:                     flags.String("base", "", "git ref to diff against (mutually exclusive with --baseline)"),
		baseline:                 flags.Bool("baseline", false, "scan every tracked file at HEAD instead of diffing against --base"),
		format:                   flags.String("format", "text", "output format: text or json"),
		scope:                    flags.String("scope", "production", "source scope: production or all"),
		buildTarget:              flags.String("build-target", "", "Go package pattern used to determine production reachability"),
		projectConfig:            flags.String("project-config", "", "repository-relative path to a project-analysis config at the selected revision; enables opt-in cross-module project facts"),
		projectLanguage:          flags.String("project-language", "go", "project-analysis language: go or typescript"),
		minSeverity:              flags.String("min-severity", "", "render only signals at or above this severity ("+severityFloorsHelp()+") and report how many were withheld; summary and coverage still describe the full analysis, and the exit status is unchanged"),
		top:                      flags.String("top", "", "render only the N highest-ranked signals (a positive integer, applied after --min-severity) and report how many were withheld; summary and coverage still describe the full analysis, and the exit status is unchanged"),
		suggestProjectConfig:     &countingBoolFlag{},
		output:                   &countingStringFlag{},
		checkProject:             &countingBoolFlag{},
		prepareCompiler:          &countingBoolFlag{},
		noInteractive:            flags.Bool("no-interactive", false, "treat this invocation as non-interactive even if a controlling terminal is attached: a scan's interactive compiler-setup and guided-policy-authoring offers are skipped in favor of the plain message-only remediation a piped invocation gets, and --suggest-project-config/--prepare-compiler refuse instead of prompting (also honored via a non-empty CI environment variable)"),
		failOnIncompleteCoverage: flags.Bool("fail-on-incomplete-coverage", false, "exit 3 when required project coverage (model or bypass, on any analyzed side) is incomplete; the report is still written to stdout"),
	}
	flags.Var(h.suggestProjectConfig, "suggest-project-config", "generate a project-config candidate JSON from Go module/workspace discovery at HEAD (requires --baseline; human-reviewed candidate only, never auto-applied); combined with --project-language typescript, runs an interactive guided-authoring session over discovered TypeScript roots instead of emitting a Go candidate directly")
	flags.Var(h.output, "output", "write the --suggest-project-config candidate to this repository-relative path instead of stdout (create-only)")
	flags.Var(h.checkProject, "check-project", "report a read-only TypeScript project-readiness result for the selected revision (requires --baseline and --project-language typescript)")
	flags.Var(h.prepareCompiler, "prepare-compiler", "run the interactive, consented mise TypeScript compiler-setup session for the selected revision, prompting before any installation (requires --baseline and --project-language typescript)")
	return h
}

func handleCodesignalHelp(args []string, flags *flag.FlagSet, suggestRequested bool, stdout, stderr *os.File) (handled bool, exitCode int) {
	for _, arg := range args {
		if arg != "--help" && arg != "-h" {
			continue
		}
		var buffer bytes.Buffer
		flags.SetOutput(&buffer)
		flags.PrintDefaults()
		if suggestRequested {
			flags.SetOutput(io.Discard)
		} else {
			flags.SetOutput(stderr)
		}
		fmt.Fprintln(stdout, codesignalUsage)
		fmt.Fprint(stdout, buffer.String())
		return true, 0
	}
	return false, 0
}

func codesignalFlagsFromHolders(h codesignalFlagHolders, setFlags map[string]bool) codesignalFlags {
	return codesignalFlags{
		base:                     *h.base,
		baseline:                 *h.baseline,
		format:                   *h.format,
		scope:                    *h.scope,
		buildTarget:              *h.buildTarget,
		projectConfig:            *h.projectConfig,
		projectLanguage:          *h.projectLanguage,
		minSeverity:              *h.minSeverity,
		minSeveritySet:           setFlags["min-severity"],
		top:                      *h.top,
		topSet:                   setFlags["top"],
		projectConfigSet:         setFlags["project-config"],
		suggestProjectConfig:     h.suggestProjectConfig.value,
		output:                   h.output.value,
		outputSet:                setFlags["output"],
		checkProject:             h.checkProject.value,
		prepareCompiler:          h.prepareCompiler.value,
		noInteractive:            *h.noInteractive,
		failOnIncompleteCoverage: *h.failOnIncompleteCoverage,
	}
}

func finishCodesignalFlagParse(flags *flag.FlagSet, h codesignalFlagHolders, stderr *os.File) (codesignalFlags, int, bool) {
	setFlags := map[string]bool{}
	flags.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	parsed := codesignalFlagsFromHolders(h, setFlags)

	if parsed.suggestProjectConfig {
		if errMsg := validateSuggestProjectConfigFlags(parsed, setFlags, flags.Args(), h.suggestProjectConfig.count, h.output.count); errMsg != "" {
			writeSuggestInvalidArguments(stderr, errMsg)
			return codesignalFlags{}, 2, false
		}
		return parsed, 0, true
	}

	if parsed.prepareCompiler {
		if errMsg := validatePrepareCompilerFlags(parsed, setFlags, flags.Args(), h.prepareCompiler.count); errMsg != "" {
			fmt.Fprintln(stderr, codesignalUsage)
			fmt.Fprintln(stderr, errMsg)
			return codesignalFlags{}, 2, false
		}
		return parsed, 0, true
	}

	if parsed.checkProject {
		if errMsg := validateCheckProjectFlags(parsed, setFlags, flags.Args(), h.checkProject.count); errMsg != "" {
			fmt.Fprintln(stderr, codesignalUsage)
			fmt.Fprintln(stderr, errMsg)
			return codesignalFlags{}, 2, false
		}
		return parsed, 0, true
	}

	if errMsg := validateCodesignalFlags(parsed, flags.Args()); errMsg != "" {
		fmt.Fprintln(stderr, codesignalUsage)
		fmt.Fprintln(stderr, errMsg)
		return codesignalFlags{}, 2, false
	}
	return parsed, 0, true
}

// --no-interactive rides with the language: only the TypeScript
// path prompts, so accepting it for a Go candidate would advertise
// a guard over a flow that never opens a session.

// validatePrepareCompilerFlags omits --format from its allowlist: this flow
// never renders a report, so there is no format to choose.

// runCheckProject dispatches `coach codesignal --check-project`: resolve
// HEAD, compute the read-only readiness result, and render it. A revision
// resolution or repository-inspection failure exits 1; an actionable
// readiness gap is still exit 0 -- the result IS the deliverable, and
// callers must read status/gaps, not the exit code.
