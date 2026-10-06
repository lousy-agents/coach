package main

import (
	"flag"
)

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

	args []string
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
		top:                      flags.String("top", "", "render only the first N signals in report order (a positive integer, applied after --min-severity; the order puts lifecycle group first, then severity) and report how many were withheld; summary and coverage still describe the full analysis, and the exit status is unchanged"),
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
