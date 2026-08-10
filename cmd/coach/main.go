// Command coach is the composition-root CLI for the coach project.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

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
	lookupProjectBackend  = func(string) codesignalcli.ProjectBackend { return nil }
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

const codesignalUsage = "usage: coach codesignal (--base <ref> | --baseline) [--format text|json] [--scope production|all] [--build-target <package>] [--project-config <path>] [--project-language go|typescript]"

type codesignalFlags struct {
	base             string
	baseline         bool
	format           string
	scope            string
	buildTarget      string
	projectConfig    string
	projectLanguage  string
	projectConfigSet bool
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

func parseCodesignalFlags(args []string, stdout, stderr *os.File) (codesignalFlags, int, bool) {
	flags := flag.NewFlagSet("codesignal", flag.ContinueOnError)
	flags.SetOutput(stderr)
	base := flags.String("base", "", "git ref to diff against (mutually exclusive with --baseline)")
	baseline := flags.Bool("baseline", false, "scan every tracked file at HEAD instead of diffing against --base")
	format := flags.String("format", "text", "output format: text or json")
	scope := flags.String("scope", "production", "source scope: production or all")
	buildTarget := flags.String("build-target", "", "Go package pattern used to determine production reachability")
	projectConfig := flags.String("project-config", "", "repository-relative path to a project-analysis config at the selected revision; enables opt-in cross-module project facts")
	projectLanguage := flags.String("project-language", "go", "project-analysis language: go or typescript")

	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			var buffer bytes.Buffer
			flags.SetOutput(&buffer)
			flags.PrintDefaults()
			flags.SetOutput(stderr)
			fmt.Fprintln(stdout, codesignalUsage)
			fmt.Fprint(stdout, buffer.String())
			return codesignalFlags{}, 0, false
		}
	}

	if err := flags.Parse(args); err != nil {
		return codesignalFlags{}, 2, false
	}

	projectConfigSet := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "project-config" {
			projectConfigSet = true
		}
	})

	parsed := codesignalFlags{
		base:             *base,
		baseline:         *baseline,
		format:           *format,
		scope:            *scope,
		buildTarget:      *buildTarget,
		projectConfig:    *projectConfig,
		projectLanguage:  *projectLanguage,
		projectConfigSet: projectConfigSet,
	}
	if errMsg := validateCodesignalFlags(parsed); errMsg != "" {
		fmt.Fprintln(stderr, codesignalUsage)
		fmt.Fprintln(stderr, errMsg)
		return codesignalFlags{}, 2, false
	}
	return parsed, 0, true
}

func validateCodesignalFlags(f codesignalFlags) string {
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
	attachProjectDiagnostic(report, diag)
	return report, projectExitCode, nil
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
	attachProjectDiagnostic(report, diag)
	return report, projectExitCode, nil
}

func attachProjectDiagnostic(report *codesignal.Report, diag *codesignal.Diagnostic) {
	if report == nil || diag == nil {
		return
	}
	report.Diagnostics = append(report.Diagnostics, *diag)
	sortReportDiagnostics(report)
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

func sortReportDiagnostics(report *codesignal.Report) {
	codesignal.SortDiagnostics(report.Diagnostics)
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
