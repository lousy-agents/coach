// Command coach is the composition-root CLI for the coach project.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// version identifies the coach binary. There is no build-time ldflags wiring
// yet; that is intentionally out of scope for this issue.
var version = "dev"

// loadProjectConfig and resolveProjectBackend are indirections over
// codesignalcli's project-analysis entry points. Tests override them to
// prove that runCodesignal's exit-code classification is driven by the
// concrete error type (errors.As), not by which call site produced the error.
var (
	loadProjectConfig     = codesignalcli.LoadProjectConfig
	resolveProjectBackend = codesignalcli.ResolveProjectBackend
	lookupProjectBackend  = func(language string) codesignalcli.ProjectBackend {
		if language == "go" {
			return codesignalcli.NewGoProjectBackend()
		}
		return nil
	}
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, topLevelUsage)
		return 2
	}

	switch args[0] {
	case "--help", "-h":
		fmt.Fprintln(stdout, topLevelUsage)
		return 0
	case "--version":
		fmt.Fprintln(stdout, version)
		return 0
	case "codesignal":
		return runCodesignal(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "%s\ncoach: unknown command %q\n", topLevelUsage, args[0])
		return 2
	}
}

const topLevelUsage = `usage: coach <command> [flags]

commands:
  codesignal   analyze production-code readiness signals in a Git diff or baseline

run "coach codesignal --help" for command-specific help.`

const codesignalUsage = "usage: coach codesignal (--base <ref> | --baseline) [--format text|json] [--scope production|all] [--build-target <package>] [--project-config <path>] [--project-language go|typescript]\n   or: coach codesignal --baseline --suggest-project-config [--output <path>]"

type codesignalFlags struct {
	base                 string
	baseline             bool
	format               string
	scope                string
	buildTarget          string
	projectConfig        string
	projectLanguage      string
	projectConfigSet     bool
	suggestProjectConfig bool
	output               string
	outputSet            bool
}

func runCodesignal(args []string, stdout, stderr *os.File) int {
	parsed, exitCode, ok := parseCodesignalFlags(args, stdout, stderr)
	if !ok {
		return exitCode
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "coach codesignal: cannot determine working directory: %s\n", err)
		return 1
	}

	if parsed.suggestProjectConfig {
		return runSuggestProjectConfig(dir, parsed, stdout, stderr)
	}

	var report *codesignal.Report
	var projectExitCode int
	if parsed.baseline {
		report, projectExitCode, err = runBaselineAnalysis(dir, parsed, stderr)
	} else {
		report, projectExitCode, err = runDiffAnalysis(dir, parsed, stderr)
	}
	if err != nil {
		return reportOperationalError(err, stderr)
	}
	if report == nil {
		return 1
	}

	if exitCode := renderReport(report, parsed.format, stdout, stderr); exitCode != 0 {
		return exitCode
	}
	return projectExitCode
}

// countingBoolFlag is a flag.Value wrapper that counts how many times Set
// was called, so parseCodesignalFlags can detect a flag supplied more than
// once (flag.FlagSet's normal Bool/String accessors silently keep only the
// last value).
type countingBoolFlag struct {
	value bool
	count int
}

func (c *countingBoolFlag) String() string {
	if c == nil {
		return "false"
	}
	return strconv.FormatBool(c.value)
}
func (c *countingBoolFlag) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	c.value = v
	c.count++
	return nil
}
func (c *countingBoolFlag) IsBoolFlag() bool { return true }

// countingStringFlag mirrors countingBoolFlag for a string-valued flag.
type countingStringFlag struct {
	value string
	count int
}

func (c *countingStringFlag) String() string {
	if c == nil {
		return ""
	}
	return c.value
}
func (c *countingStringFlag) Set(s string) error {
	c.value = s
	c.count++
	return nil
}

// suggestProjectConfigRequested reports whether --suggest-project-config
// appears anywhere in args as a bare flag or an explicit true-ish value,
// independent of whether the rest of args parses successfully. It backs
// the invalid-arguments envelope path: an unknown flag or malformed
// argument list combined with a requested --suggest-project-config must
// still surface the project_config_suggestion_invalid_arguments envelope
// (not the standard flag-package usage text), regardless of where in args
// the bad token appears relative to --suggest-project-config.
//
// An explicit --suggest-project-config=false does not count as requested:
// this pre-scan runs before flags.Parse, so it must honor the flag's own
// boolean value rather than reacting to its mere presence, or an unrelated
// malformed flag combined with a deliberately disabled
// --suggest-project-config would wrongly surface the suggestion envelope
// instead of the plain usage-text error every other codesignal flag error
// produces. A value that fails strconv.ParseBool is treated as requested,
// same as before this distinction existed: flags.Parse will itself reject
// it as a malformed --suggest-project-config value, which is exactly the
// case this envelope exists to report.
func suggestProjectConfigRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--suggest-project-config" || arg == "-suggest-project-config" {
			return true
		}
		if value, ok := suggestProjectConfigFlagValue(arg); ok {
			if requested, err := strconv.ParseBool(value); err == nil && !requested {
				continue
			}
			return true
		}
	}
	return false
}

// suggestProjectConfigFlagValue extracts <value> from a
// --suggest-project-config=<value>/-suggest-project-config=<value> token.
func suggestProjectConfigFlagValue(arg string) (string, bool) {
	for _, prefix := range []string{"--suggest-project-config=", "-suggest-project-config="} {
		if strings.HasPrefix(arg, prefix) {
			return arg[len(prefix):], true
		}
	}
	return "", false
}

func writeSuggestInvalidArguments(stderr *os.File, message string) {
	stderr.Write(codesignalcli.InvalidArgumentsSuggestionEnvelope(message))
}

type codesignalFlagHolders struct {
	base                 *string
	baseline             *bool
	format               *string
	scope                *string
	buildTarget          *string
	projectConfig        *string
	projectLanguage      *string
	suggestProjectConfig *countingBoolFlag
	output               *countingStringFlag
}

func registerCodesignalFlags(flags *flag.FlagSet) codesignalFlagHolders {
	h := codesignalFlagHolders{
		base:                 flags.String("base", "", "git ref to diff against (mutually exclusive with --baseline)"),
		baseline:             flags.Bool("baseline", false, "scan every tracked file at HEAD instead of diffing against --base"),
		format:               flags.String("format", "text", "output format: text or json"),
		scope:                flags.String("scope", "production", "source scope: production or all"),
		buildTarget:          flags.String("build-target", "", "Go package pattern used to determine production reachability"),
		projectConfig:        flags.String("project-config", "", "repository-relative path to a project-analysis config at the selected revision; enables opt-in cross-module project facts"),
		projectLanguage:      flags.String("project-language", "go", "project-analysis language: go or typescript"),
		suggestProjectConfig: &countingBoolFlag{},
		output:               &countingStringFlag{},
	}
	flags.Var(h.suggestProjectConfig, "suggest-project-config", "generate a project-config candidate JSON from Go module/workspace discovery at HEAD (requires --baseline; human-reviewed candidate only, never auto-applied)")
	flags.Var(h.output, "output", "write the --suggest-project-config candidate to this repository-relative path instead of stdout (create-only)")
	return h
}

// handleCodesignalHelp prints codesignal usage + defaults when --help/-h is
// present and restores the FlagSet output sink used for subsequent Parse.
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
		base:                 *h.base,
		baseline:             *h.baseline,
		format:               *h.format,
		scope:                *h.scope,
		buildTarget:          *h.buildTarget,
		projectConfig:        *h.projectConfig,
		projectLanguage:      *h.projectLanguage,
		projectConfigSet:     setFlags["project-config"],
		suggestProjectConfig: h.suggestProjectConfig.value,
		output:               h.output.value,
		outputSet:            setFlags["output"],
	}
}

// finishCodesignalFlagParse validates the post-Parse flag combination for
// either the suggest-project-config path or the normal codesignal path.
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

	if errMsg := validateCodesignalFlags(parsed); errMsg != "" {
		fmt.Fprintln(stderr, codesignalUsage)
		fmt.Fprintln(stderr, errMsg)
		return codesignalFlags{}, 2, false
	}
	return parsed, 0, true
}

func parseCodesignalFlags(args []string, stdout, stderr *os.File) (codesignalFlags, int, bool) {
	suggestRequested := suggestProjectConfigRequested(args)

	flags := flag.NewFlagSet("codesignal", flag.ContinueOnError)
	if suggestRequested {
		flags.SetOutput(io.Discard)
	} else {
		flags.SetOutput(stderr)
	}
	holders := registerCodesignalFlags(flags)

	if handled, code := handleCodesignalHelp(args, flags, suggestRequested, stdout, stderr); handled {
		return codesignalFlags{}, code, false
	}

	if err := flags.Parse(args); err != nil {
		if suggestRequested {
			writeSuggestInvalidArguments(stderr, fmt.Sprintf("coach codesignal --suggest-project-config: invalid flags (project_config_suggestion_invalid_arguments): %s", err))
			return codesignalFlags{}, 2, false
		}
		return codesignalFlags{}, 2, false
	}

	return finishCodesignalFlagParse(flags, holders, stderr)
}

// sortedFlagNames returns setFlags' keys in sorted order, so a caller that
// reports the first disallowed flag gets a deterministic result regardless
// of Go's randomized map iteration order.
func sortedFlagNames(setFlags map[string]bool) []string {
	names := make([]string, 0, len(setFlags))
	for name := range setFlags {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// validateSuggestProjectConfigFlags rejects any flag combination the
// --suggest-project-config contract (issue #220) forbids outright: it
// never picks a precedence between --suggest-project-config and a
// conflicting flag, it rejects the combination. Rather than enumerating
// every flag known to conflict (which silently stops protecting a newly
// added codesignal flag), it walks setFlags -- the flags actually supplied
// -- against the fixed allowlist the contract describes ({baseline,
// output, suggest-project-config}), so any other flag is rejected by
// construction.
func validateSuggestProjectConfigFlags(f codesignalFlags, setFlags map[string]bool, positional []string, suggestCount, outputCount int) string {
	if suggestCount > 1 {
		return "coach: --suggest-project-config may only be provided once (project_config_suggestion_invalid_arguments)"
	}
	if outputCount > 1 {
		return "coach: --output may only be provided once (project_config_suggestion_invalid_arguments)"
	}
	if !f.baseline {
		return "coach: --suggest-project-config requires --baseline (project_config_suggestion_invalid_arguments)"
	}
	allowedWithSuggest := map[string]bool{"suggest-project-config": true, "output": true, "baseline": true}
	for _, name := range sortedFlagNames(setFlags) {
		if !allowedWithSuggest[name] {
			return fmt.Sprintf("coach: --suggest-project-config cannot be combined with --%s (project_config_suggestion_invalid_arguments)", name)
		}
	}
	if len(positional) > 0 {
		return "coach: --suggest-project-config does not accept positional arguments (project_config_suggestion_invalid_arguments)"
	}
	return ""
}

// runSuggestProjectConfig dispatches `coach codesignal --baseline
// --suggest-project-config`: it never builds a codesignal.Report, unlike
// the diff/baseline analysis paths above.
func runSuggestProjectConfig(dir string, f codesignalFlags, stdout, stderr *os.File) int {
	result := codesignalcli.SuggestProjectConfig(dir, f.output, f.outputSet)
	if len(result.Envelope) > 0 {
		if _, writeErr := stderr.Write(result.Envelope); writeErr != nil {
			fmt.Fprintf(stderr, "coach codesignal: writing diagnostic: %s\n", writeErr)
			return 1
		}
	}
	if result.ExitCode == 0 && len(result.Candidate) > 0 {
		if _, writeErr := stdout.Write(result.Candidate); writeErr != nil {
			fmt.Fprintf(stderr, "coach codesignal: writing candidate: %s\n", writeErr)
			return 1
		}
	}
	return result.ExitCode
}

func validateCodesignalFlags(f codesignalFlags) string {
	if f.outputSet {
		return "coach: --output requires --suggest-project-config"
	}
	if f.baseline && f.base != "" {
		return "coach: --baseline and --base are mutually exclusive: choose a Repository Baseline scan (--baseline) or a diff comparison (--base), not both"
	}
	if !f.baseline && f.base == "" {
		return "coach: missing required --base flag"
	}
	if f.format != "text" && f.format != "json" {
		return fmt.Sprintf("coach: invalid --format value %q: must be \"text\" or \"json\"", f.format)
	}
	if f.scope != "production" && f.scope != "all" {
		return fmt.Sprintf("coach: invalid --scope value %q: must be \"production\" or \"all\"", f.scope)
	}
	if f.projectLanguage != "go" && f.projectLanguage != "typescript" {
		return fmt.Sprintf("coach: invalid --project-language value %q: must be \"go\" or \"typescript\"", f.projectLanguage)
	}
	return ""
}

func runBaselineAnalysis(dir string, f codesignalFlags, stderr *os.File) (*codesignal.Report, int, error) {
	revisionSHA, err := codesignalcli.ResolveBaselineRevision(dir)
	if err != nil {
		return nil, 0, err
	}
	discovered, coverage, err := codesignalcli.DiscoverTrackedFiles(dir, revisionSHA)
	if err != nil {
		return nil, 0, err
	}
	kept, excluded, err := codesignalcli.ApplyBaselineSourceScope(dir, revisionSHA, f.buildTarget, f.scope, discovered)
	if err != nil {
		return nil, 0, err
	}
	coverage.Excluded = excluded

	project, diag, projectExitCode, opErr := prepareProjectAnalysis(dir, revisionSHA, f.projectConfigSet, f.projectConfig, f.projectLanguage)
	if opErr != nil {
		return nil, 0, opErr
	}
	report, err := codesignalcli.AnalyzeBaseline(context.Background(), dir, revisionSHA, kept, nil, coverage, project)
	if err != nil {
		fmt.Fprintf(stderr, "coach codesignal: analysis failed: %s\n", err)
		return nil, 0, nil
	}
	return withProjectDiagnostic(report, diag), projectExitCode, nil
}

func runDiffAnalysis(dir string, f codesignalFlags, stderr *os.File) (*codesignal.Report, int, error) {
	headSHA, mergeBaseSHA, err := codesignalcli.ResolveRevisions(dir, f.base)
	if err != nil {
		return nil, 0, err
	}

	selected, diagnostics, err := codesignalcli.SelectChangedFiles(dir, mergeBaseSHA)
	if err != nil {
		return nil, 0, err
	}
	selected, excluded, err := codesignalcli.ApplySourceScope(dir, headSHA, f.buildTarget, f.scope, selected)
	if err != nil {
		return nil, 0, err
	}

	project, diag, projectExitCode, opErr := prepareProjectAnalysis(dir, headSHA, f.projectConfigSet, f.projectConfig, f.projectLanguage)
	if opErr != nil {
		return nil, 0, opErr
	}
	report, err := codesignalcli.AnalyzeChanges(context.Background(), dir, headSHA, mergeBaseSHA, selected, diagnostics, f.scope, excluded, project)
	if err != nil {
		fmt.Fprintf(stderr, "coach codesignal: analysis failed: %s\n", err)
		return nil, 0, nil
	}
	return withProjectDiagnostic(report, diag), projectExitCode, nil
}

func withProjectDiagnostic(report *codesignal.Report, diag *codesignal.Diagnostic) *codesignal.Report {
	if report == nil || diag == nil {
		return report
	}
	out := *report
	out.Diagnostics = append(append([]codesignal.Diagnostic(nil), report.Diagnostics...), *diag)
	codesignal.SortDiagnostics(out.Diagnostics)
	return &out
}

// prepareProjectAnalysis resolves the typed project handoff. When the flag is
// omitted, all results are zero. Config/backend failures return a diagnostic
// and exit code while keeping project nil so file-local analysis stays schema-1.
// Unexpected error types return opErr for the operational path (no report).
func prepareProjectAnalysis(dir, revision string, projectConfigSet bool, configPath, language string) (*codesignalcli.ProjectAnalysis, *codesignal.Diagnostic, int, error) {
	if !projectConfigSet {
		return nil, nil, 0, nil
	}
	config, err := loadProjectConfig(dir, revision, configPath)
	if err != nil {
		var configErr *codesignalcli.ProjectConfigError
		if !errors.As(err, &configErr) {
			return nil, nil, 0, err
		}
		return nil, &codesignal.Diagnostic{
			Kind:    "project_config_invalid",
			Path:    configPath,
			Message: configErr.Message,
		}, 2, nil
	}
	if err := resolveProjectBackend(language); err != nil {
		var backendErr *codesignalcli.ProjectBackendUnavailableError
		if !errors.As(err, &backendErr) {
			return nil, nil, 0, err
		}
		return nil, &codesignal.Diagnostic{
			Kind:    "project_backend_unavailable",
			Path:    configPath,
			Message: backendErr.Message,
		}, 3, nil
	}
	backend := lookupProjectBackend(language)
	if backend == nil {
		return nil, &codesignal.Diagnostic{
			Kind:    "project_backend_unavailable",
			Path:    configPath,
			Message: fmt.Sprintf("coach codesignal: no project-analysis backend is available for language %q yet (project_backend_unavailable)", language),
		}, 3, nil
	}
	return &codesignalcli.ProjectAnalysis{
		ConfigPath:   configPath,
		Language:     language,
		Config:       append(json.RawMessage(nil), config...),
		ConfigDigest: codesignalcli.ConfigDigest(config),
		Backend:      backend,
	}, nil, 0, nil
}

func renderReport(report *codesignal.Report, format string, stdout, stderr *os.File) int {
	if format == "json" {
		encoded, err := codesignalcli.RenderJSON(report)
		if err != nil {
			fmt.Fprintf(stderr, "coach codesignal: encoding report: %s\n", err)
			return 1
		}
		if _, err := stdout.Write(encoded); err != nil {
			fmt.Fprintf(stderr, "coach codesignal: writing report: %s\n", err)
			return 1
		}
		return 0
	}

	if _, err := fmt.Fprint(stdout, codesignalcli.RenderText(report)); err != nil {
		fmt.Fprintf(stderr, "coach codesignal: writing report: %s\n", err)
		return 1
	}
	return 0
}

func reportOperationalError(err error, stderr *os.File) int {
	var opErr *codesignalcli.OperationalError
	if errors.As(err, &opErr) {
		fmt.Fprintln(stderr, opErr.Message)
		return 1
	}
	fmt.Fprintln(stderr, err)
	return 1
}
