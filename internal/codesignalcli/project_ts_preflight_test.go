package codesignalcli

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAppendedRemediationLine(t *testing.T) {
	const line = "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript"

	t.Run("without a controlling terminal prints the remediation line", func(t *testing.T) {
		if got := AppendedRemediationLine(false, "typescript", line); got != line {
			t.Fatalf("AppendedRemediationLine(false, %q, line) = %q, want %q", "typescript", got, line)
		}
	})
	t.Run("a TypeScript controlling terminal withholds the line because the interactive offer owns it", func(t *testing.T) {
		if got := AppendedRemediationLine(true, "typescript", line); got != "" {
			t.Fatalf("AppendedRemediationLine(true, %q, line) = %q, want empty: the interactive setup offer owns the controlling-terminal case for typescript", "typescript", got)
		}
	})
	t.Run("a Go controlling terminal still prints the line because Go has no interactive offer", func(t *testing.T) {
		if got := AppendedRemediationLine(true, "go", line); got != line {
			t.Fatalf("AppendedRemediationLine(true, %q, line) = %q, want %q: go has no interactive setup offer to own the controlling-terminal case", "go", got, line)
		}
	})
}

func TestPrepareCompilerRemediation(t *testing.T) {
	if got, want := PrepareCompilerRemediation(GapTypescriptCompilerMissing, "project.json"), "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want %q", GapTypescriptCompilerMissing, "project.json", got, want)
	}
	if got, want := PrepareCompilerRemediation(GapTypescriptVersionMismatch, ""), "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, \"\") = %q, want %q", GapTypescriptVersionMismatch, got, want)
	}
	if got, want := PrepareCompilerRemediation(GapTypescriptVersionConflict, "project.json"), "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want %q", GapTypescriptVersionConflict, "project.json", got, want)
	}
	if got := PrepareCompilerRemediation(GapNodeMissing, "project.json"); got != "" {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want empty: Coach has no executable remediation for a runtime-boundary gap", GapNodeMissing, "project.json", got)
	}
	if got := PrepareCompilerRemediation(GapNodeUnsupported, "project.json"); got != "" {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want empty: Coach has no executable remediation for a runtime-boundary gap", GapNodeUnsupported, "project.json", got)
	}
}

// TestPrepareCompilerRemediationOffersOnlyExecutablePrepareCompilerGaps pins
// AC-SET-10's runtime-boundary installer suppression against every gap code
// gapCodeTable currently defines, via a literal per-code expectation rather
// than an expression derived from production code: deriving "should offer a
// command" from nextActionExecutable/gapCodeTable itself (as
// PrepareCompilerRemediation does) would make the assertion tautological and
// unable to catch a mutation to either.
func TestPrepareCompilerRemediationOffersOnlyExecutablePrepareCompilerGaps(t *testing.T) {
	wantCommand := map[string]bool{
		GapUnsupportedRepositoryShape:        false,
		GapNodeMissing:                       false,
		GapNodeUnsupported:                   false,
		GapNodeUnverifiable:                  false,
		GapTypescriptCompilerMissing:         true,
		GapTypescriptVersionMismatch:         true,
		GapTypescriptVersionConflict:         true,
		GapPackageManagerAmbiguous:           false,
		GapPackageManagerConfigUnverifiable:  false,
		GapPackageManagerVersionUnverifiable: false,
		GapPackageManagerVersionUnsupported:  false,
		GapPolicyMissing:                     false,
		GapPolicyInvalid:                     false,
	}

	if len(wantCommand) != len(gapCodeTable) {
		t.Fatalf("wantCommand has %d entries, gapCodeTable has %d: add the missing gap code to wantCommand with an explicit executable/non-executable decision", len(wantCommand), len(gapCodeTable))
	}
	for code := range gapCodeTable {
		if _, ok := wantCommand[code]; !ok {
			t.Fatalf("gapCodeTable defines %q but wantCommand does not: add an explicit executable/non-executable decision for it", code)
		}
	}

	for code, wantExecutable := range wantCommand {
		got := PrepareCompilerRemediation(code, "project.json")
		if wantExecutable {
			if got == "" {
				t.Errorf("PrepareCompilerRemediation(%q, ...) = \"\", want a non-empty --prepare-compiler command", code)
			}
			continue
		}
		if got != "" {
			t.Errorf("PrepareCompilerRemediation(%q, ...) = %q, want empty: AC-SET-10 forbids offering a command for a non-prepare-compiler gap", code, got)
		}
	}
}

// TestPrepareCompilerRemediationWithReadinessWithholdsADeadEndCommand pins
// O2: even for a gap code whose next action is the executable
// prepare-compiler kind, the command must be withheld unless the attached
// readiness snapshot offers a choice --prepare-compiler itself can execute.
// That flag runs mise scopes only (filterMiseChoiceKinds), so a menu whose
// sole executable entry is project_package is as much a dead end as an empty
// one: the named command would open, discard the only offered choice, and
// exit 0 reporting it had nothing to set up while the compiler is still
// missing.
func TestPrepareCompilerRemediationWithReadinessWithholdsADeadEndCommand(t *testing.T) {
	nothingOffered := &ReadinessResult{
		Checks: ReadinessChecks{Compiler: ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing}},
	}
	if got := PrepareCompilerRemediationWithReadiness(GapTypescriptCompilerMissing, "project.json", nothingOffered); got != "" {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(dead-end menu) = %q, want empty", got)
	}

	projectPackageOnly := &ReadinessResult{
		Checks: ReadinessChecks{
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}
	if got := PrepareCompilerRemediationWithReadiness(GapTypescriptCompilerMissing, "project.json", projectPackageOnly); got != "" {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(project_package-only menu) = %q, want empty: --prepare-compiler runs mise scopes only, so it would exit 0 having set nothing up", got)
	}

	miseOffered := &ReadinessResult{
		Checks:      ReadinessChecks{Compiler: ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing}},
		MiseChoices: []ReadinessMiseChoice{{Kind: compilerOriginMiseProject, Verified: true}},
	}
	want := "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"
	if got := PrepareCompilerRemediationWithReadiness(GapTypescriptCompilerMissing, "project.json", miseOffered); got != want {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(verified mise scope) = %q, want %q", got, want)
	}

	if got := PrepareCompilerRemediationWithReadiness(GapNodeMissing, "project.json", miseOffered); got != "" {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(%q, ...) = %q, want empty: a runtime-boundary gap code must still be withheld regardless of readiness", GapNodeMissing, got)
	}

	if got, want := PrepareCompilerRemediationWithReadiness(GapTypescriptCompilerMissing, "project.json", nil), PrepareCompilerRemediation(GapTypescriptCompilerMissing, "project.json"); got != want {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(nil readiness) = %q, want the plain PrepareCompilerRemediation fallback %q", got, want)
	}
}

func TestSuggestProjectConfigRemediation(t *testing.T) {
	t.Run("TypeScript names the guided-authoring invocation plus the no-terminal path", func(t *testing.T) {
		ts := SuggestProjectConfigRemediation("typescript")
		if !strings.Contains(ts, "coach codesignal --baseline --suggest-project-config --project-language typescript") {
			t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want it to still name the guided-authoring invocation", "typescript", ts)
		}
		if !strings.Contains(ts, "requires a controlling terminal") || !strings.Contains(ts, "draft the schema-1 project-config document yourself") {
			t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want it to state the terminal requirement and the no-terminal path, since this line is printed only when no controlling terminal is available", "typescript", ts)
		}
	})
	t.Run("Go is offered only the non-interactive suggest command", func(t *testing.T) {
		if got, want := SuggestProjectConfigRemediation("go"), "coach codesignal --baseline --suggest-project-config"; got != want {
			t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want %q: a go scan must never be offered the TypeScript-only guided-authoring command (AC-2)", "go", got, want)
		}
	})
}

// TestAlsoFailingGapLinesReportsEveryGapWithoutClaimingItSurvives pins
// AC-SET-13's report-all-gaps clause: every non-policy gap readiness.Gaps
// carries is reported, in readiness's own order, and the line names the
// --check-project rerun with no hedge about whether the gap outlives the
// policy failure. Since R1, checkProjectShape and checkPackageManager
// report not_checked instead of guessing from the worktree root when the
// policy has failed, so nothing left in readiness.Gaps here is an artifact
// of that guess to hedge about in the first place -- a package-manager
// version is probed from the manager binary and never depends on roots,
// and a shape finding that would have needed the guess is absent from
// Gaps entirely rather than present and uncertain.
func TestAlsoFailingGapLinesReportsEveryGapWithoutClaimingItSurvives(t *testing.T) {
	readiness := &ReadinessResult{
		Checks: ReadinessChecks{Policy: ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing}},
		Gaps: []ReadinessGap{
			{Code: GapPolicyMissing},
			{Code: GapNodeUnsupported},
			{Code: GapTypescriptCompilerMissing},
			{Code: GapPackageManagerVersionUnsupported},
		},
	}
	got := AlsoFailingGapLines(readiness, "project.json")
	want := []string{GapNodeUnsupported, GapTypescriptCompilerMissing, GapPackageManagerVersionUnsupported}
	if len(got) != len(want) {
		t.Fatalf("AlsoFailingGapLines() = %q, want one line for each of %q", got, want)
	}
	for i, code := range want {
		if !strings.HasPrefix(got[i], code+":") {
			t.Fatalf("AlsoFailingGapLines()[%d] = %q, want the %q gap in readiness's own frozen order", i, got[i], code)
		}
		if strings.Contains(got[i], "once the policy above is authored and committed") || strings.Contains(got[i], "checked without the policy's selected roots") {
			t.Fatalf("AlsoFailingGapLines()[%d] = %q: the line must not hedge about whether the gap outlives the policy failure -- R1 already withholds any gap that guess would have produced", i, got[i])
		}
		if !strings.Contains(got[i], "run coach codesignal --baseline --check-project") {
			t.Fatalf("AlsoFailingGapLines()[%d] = %q, want it to name the --check-project rerun", i, got[i])
		}
	}

	t.Run("a policy-only gap list prints nothing extra", func(t *testing.T) {
		if got := AlsoFailingGapLines(&ReadinessResult{Gaps: []ReadinessGap{{Code: GapPolicyMissing}}}, "project.json"); len(got) != 0 {
			t.Fatalf("AlsoFailingGapLines(policy gap only) = %q, want none: the policy failure is already printed as the scan's own message", got)
		}
	})
	t.Run("nil readiness prints nothing extra", func(t *testing.T) {
		if got := AlsoFailingGapLines(nil, "project.json"); len(got) != 0 {
			t.Fatalf("AlsoFailingGapLines(nil readiness) = %q, want none", got)
		}
	})
}

func TestAlsoFailingGapLinesFollowsReadinessGaps(t *testing.T) {
	t.Run("a shaped failing compiler is reported from gaps", func(t *testing.T) {
		failingCompilerShaped := &ReadinessResult{
			Checks: ReadinessChecks{
				ProjectShape: ReadinessCheck{State: ReadinessPass},
				Compiler:     ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing},
			},
			Gaps: []ReadinessGap{{Code: GapTypescriptCompilerMissing}},
		}
		want := "typescript_compiler_missing: also failing, run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"
		if got := AlsoFailingGapLines(failingCompilerShaped, "project.json"); len(got) != 1 || got[0] != want {
			t.Fatalf("AlsoFailingGapLines(shaped) = %q, want exactly [%q]", got, want)
		}
	})

	t.Run("a passing compiler with no gaps reports nothing", func(t *testing.T) {
		passingCompiler := &ReadinessResult{
			Checks: ReadinessChecks{
				ProjectShape: ReadinessCheck{State: ReadinessPass},
				Compiler:     ReadinessCheck{State: ReadinessPass},
			},
		}
		if got := AlsoFailingGapLines(passingCompiler, "project.json"); len(got) != 0 {
			t.Fatalf("AlsoFailingGapLines(passing compiler) = %q, want none", got)
		}
	})

	t.Run("an unsupported repository shape still reports every gap readiness names", func(t *testing.T) {
		notShapedButGapped := &ReadinessResult{
			Checks: ReadinessChecks{
				ProjectShape: ReadinessCheck{State: ReadinessFail, Code: GapUnsupportedRepositoryShape},
				Compiler:     ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing},
			},
			Gaps: []ReadinessGap{{Code: GapUnsupportedRepositoryShape}, {Code: GapTypescriptCompilerMissing}},
		}
		got := AlsoFailingGapLines(notShapedButGapped, "project.json")
		if len(got) != 2 || !strings.HasPrefix(got[0], GapUnsupportedRepositoryShape+":") || !strings.HasPrefix(got[1], GapTypescriptCompilerMissing+":") {
			t.Fatalf("AlsoFailingGapLines(unsupported repository shape, still gapped) = %q, want both gaps in readiness's own order", got)
		}
	})

	t.Run("a failing check absent from gaps is not reported", func(t *testing.T) {
		compilerFailingButNotInGaps := &ReadinessResult{
			Checks: ReadinessChecks{
				Compiler: ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing},
			},
		}
		if got := AlsoFailingGapLines(compilerFailingButNotInGaps, "project.json"); len(got) != 0 {
			t.Fatalf("AlsoFailingGapLines(failing check absent from gaps[]) = %q, want none", got)
		}
	})
}
func TestWrapProjectConfigErrorWithReadinessLeavesOtherErrorsUnchanged(t *testing.T) {
	plain := &OperationalError{Message: "boom"}
	if got := WrapProjectConfigErrorWithReadiness(plain, ".", "HEAD", "project.json"); got != error(plain) {
		t.Fatalf("WrapProjectConfigErrorWithReadiness(non-ProjectConfigError) = %v, want the original error unchanged", got)
	}
}

func TestWrapCompilerUnresolvedErrorWithReadinessLeavesOtherErrorsUnchanged(t *testing.T) {
	plain := &OperationalError{Message: "boom"}
	if got := WrapCompilerUnresolvedErrorWithReadiness(plain, ".", "HEAD", "project.json"); got != error(plain) {
		t.Fatalf("WrapCompilerUnresolvedErrorWithReadiness(non-CompilerUnresolvedError) = %v, want the original error unchanged", got)
	}
}

// TestWrapCompilerUnresolvedErrorWithReadinessUnwrapsToThePlainError pins
// classifyAnalysisError's own no-controlling-terminal fallback: it locates a
// *CompilerUnresolvedError via errors.As without any change, because
// CompilerUnresolvedErrorWithReadiness.Unwrap returns the original value
// unchanged.
func TestWrapCompilerUnresolvedErrorWithReadinessUnwrapsToThePlainError(t *testing.T) {
	repo := newTempGitRepoT(t)
	head := commitFileT(t, repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

	original := &CompilerUnresolvedError{Code: GapTypescriptCompilerMissing, ConfigPath: "project.json"}
	wrapped := WrapCompilerUnresolvedErrorWithReadiness(original, repo, head, "project.json")

	var withReadiness *CompilerUnresolvedErrorWithReadiness
	if !errors.As(wrapped, &withReadiness) {
		t.Fatalf("WrapCompilerUnresolvedErrorWithReadiness did not produce a *CompilerUnresolvedErrorWithReadiness: %v", wrapped)
	}
	if withReadiness.Readiness == nil {
		t.Fatalf("CompilerUnresolvedErrorWithReadiness.Readiness is nil, want a populated snapshot")
	}
	if withReadiness.Revision != head || withReadiness.ConfigPath != "project.json" {
		t.Fatalf("CompilerUnresolvedErrorWithReadiness = {Revision: %q, ConfigPath: %q}, want {%q, %q}", withReadiness.Revision, withReadiness.ConfigPath, head, "project.json")
	}

	var plain *CompilerUnresolvedError
	if !errors.As(wrapped, &plain) {
		t.Fatalf("errors.As could not unwrap back to the original *CompilerUnresolvedError")
	}
	if plain != original {
		t.Fatalf("errors.As unwrapped to a different *CompilerUnresolvedError value than the one passed in")
	}
}

// TestRunCompilerSetupOfferReportsNoChoicesOffered pins the fail-fast cases
// where RunCompilerSetupOffer must never print a single byte of prompt: a
// nil readiness snapshot, a readiness result whose compiler check is not
// actually failing, and a failing compiler check for which
// AvailableSetupChoices offers only its always-appended cancel entry (no
// package manager, no verified mise scope).
func TestRunCompilerSetupOfferReportsNoChoicesOffered(t *testing.T) {
	cases := map[string]*ReadinessResult{
		"nil readiness":          nil,
		"compiler check passing": {Checks: ReadinessChecks{Policy: ReadinessCheck{State: ReadinessPass}, Compiler: ReadinessCheck{State: ReadinessPass}}},
		"compiler check failing but no choice is offerable": {
			Checks: ReadinessChecks{Policy: ReadinessCheck{State: ReadinessPass}, Compiler: ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing}},
		},
	}
	for name, readiness := range cases {
		t.Run(name, func(t *testing.T) {
			var out strings.Builder
			result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", GapTypescriptCompilerMissing, readiness, strings.NewReader(""), &out)
			if !result.NoChoicesOffered {
				t.Fatalf("NoChoicesOffered = false, want true: %+v", result)
			}
			if result.Cancelled || result.Succeeded || result.Choice != "" {
				t.Fatalf("expected every other outcome field to stay zero, got %+v", result)
			}
			if out.Len() != 0 {
				t.Fatalf("expected no prompt output at all, got %q", out.String())
			}
		})
	}
}

// TestRunCompilerSetupOfferCancelsOnUnreadableSelection pins the fail-closed
// default: an input stream that produces no answer at all (EOF) must cancel
// rather than falling back to any offered choice. The fixture must offer a
// genuine, non-cancel choice (an installable manifest declaration plus a
// passing package-manager check) or the prompt this pins would never open at
// all.
func TestRunCompilerSetupOfferCancelsOnUnreadableSelection(t *testing.T) {
	readiness := &ReadinessResult{
		Checks: ReadinessChecks{
			Policy:         ReadinessCheck{State: ReadinessPass},
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", GapTypescriptCompilerMissing, readiness, strings.NewReader(""), &out)
	if !result.Cancelled {
		t.Fatalf("Cancelled = false, want true: %+v", result)
	}
	if result.Succeeded || result.Choice != "" {
		t.Fatalf("expected every other outcome field to stay zero, got %+v", result)
	}
	if !strings.Contains(out.String(), "cancel") {
		t.Fatalf("expected the menu to have offered a cancel choice, got %q", out.String())
	}
}

// TestRepositoryRelativeChangedPathsOnlyRewritesTheResidueUnknownFallback
// pins both branches of repositoryRelativeChangedPaths: an ordinary
// git-status disclosure (already repository-relative) passes through
// unchanged, while setupResidueChangedPaths' documented residueUnknown
// fallback -- which names workingDirectory itself, an absolute path -- is
// rewritten relative to the worktree root before it can reach the customer.
func TestRepositoryRelativeChangedPathsOnlyRewritesTheResidueUnknownFallback(t *testing.T) {
	root := newTempGitRepoT(t)
	workingDirectory := filepath.Join(root, "packages", "app")

	cases := []struct {
		name           string
		changedPaths   []string
		residueUnknown bool
		want           []string
	}{
		{
			name:           "a real git-status disclosure is already repository-relative and passes through unchanged",
			changedPaths:   []string{"packages/app/node_modules/"},
			residueUnknown: false,
			want:           []string{"packages/app/node_modules/"},
		},
		{
			name:           "residueUnknown's absolute workingDirectory fallback is rewritten repository-relative",
			changedPaths:   []string{workingDirectory},
			residueUnknown: true,
			want:           []string{filepath.Join("packages", "app")},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := repositoryRelativeChangedPaths(root, c.changedPaths, c.residueUnknown)
			if !slices.Equal(got, c.want) {
				t.Fatalf("repositoryRelativeChangedPaths(%q, %v, %v) = %v, want %v", root, c.changedPaths, c.residueUnknown, got, c.want)
			}
		})
	}
}

// TestRunCompilerSetupOfferCancelsOnExplicitCancelChoice pins that typing
// the literal "cancel" choice, not merely an unreadable/blank answer,
// selects the same Cancelled outcome with no Choice recorded. The fixture
// must offer a genuine, non-cancel choice or the prompt this pins would
// never open at all.
func TestRunCompilerSetupOfferCancelsOnExplicitCancelChoice(t *testing.T) {
	readiness := &ReadinessResult{
		Checks: ReadinessChecks{
			Policy:         ReadinessCheck{State: ReadinessPass},
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", GapTypescriptCompilerMissing, readiness, strings.NewReader("cancel\n"), &out)
	if !result.Cancelled {
		t.Fatalf("Cancelled = false, want true: %+v", result)
	}
	if result.Choice != "" {
		t.Fatalf("Choice = %q, want empty on a cancelled selection", result.Choice)
	}
}

// TestRunCompilerSetupOfferRequiresPolicyFirst pins O3's defense-in-depth
// precondition, mirroring RunPrepareCompilerMiseSetup's own
// TestRunPrepareCompilerMiseSetupRequiresPolicyFirst: even when the readiness
// snapshot's compiler check genuinely offers an executable menu entry,
// RunCompilerSetupOffer must refuse before ever printing a prompt while
// readiness.Checks.Policy has not passed.
func TestRunCompilerSetupOfferRequiresPolicyFirst(t *testing.T) {
	readiness := &ReadinessResult{
		Checks: ReadinessChecks{
			Policy:         ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", GapTypescriptCompilerMissing, readiness, strings.NewReader(""), &out)

	if !result.PolicyRequired {
		t.Fatalf("PolicyRequired = false, want true: %+v", result)
	}
	if result.NoChoicesOffered || result.Cancelled || result.Succeeded || result.Choice != "" {
		t.Fatalf("expected every other outcome field to stay zero, got %+v", result)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output at all while a policy gap coexists with a compiler gap, got %q", out.String())
	}
}

// TestRunCompilerSetupOfferNeverOpensForARuntimeBoundaryGap pins AC-13/
// AC-SET-10's interactive-path gate: RunCompilerSetupOffer must withhold the
// offer -- printing nothing at all -- for any gapCode whose next action is
// not the executable prepare-compiler kind, even when the readiness snapshot
// passed alongside it independently offers a genuine, installable menu
// entry. gapCode and readiness.Checks.Compiler.Code are two independent
// CheckProjectReadiness reads (project_readiness.go resolves Node and the
// compiler separately), so a runtime-boundary gap can coexist with an
// installable compiler menu; this is the fixture shape that previously drove
// the interactive menu open for node_missing/node_unsupported. A literal
// per-code expectation, not one derived from gapCodeIsExecutablePrepareCompiler
// itself, so a mutation to either cannot pass by construction.
func TestRunCompilerSetupOfferNeverOpensForARuntimeBoundaryGap(t *testing.T) {
	readiness := &ReadinessResult{
		Checks: ReadinessChecks{
			Policy:         ReadinessCheck{State: ReadinessPass},
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}

	wantOffered := map[string]bool{
		GapNodeMissing:               false,
		GapNodeUnsupported:           false,
		GapNodeUnverifiable:          false,
		GapTypescriptCompilerMissing: true,
	}

	for gapCode, offered := range wantOffered {
		t.Run(gapCode, func(t *testing.T) {
			var out strings.Builder
			result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", gapCode, readiness, strings.NewReader("cancel\n"), &out)
			if offered {
				if result.NoChoicesOffered {
					t.Fatalf("NoChoicesOffered = true for executable gap %q, want the menu to have opened: %+v", gapCode, result)
				}
				if out.Len() == 0 {
					t.Fatalf("expected the menu to have been printed for executable gap %q", gapCode)
				}
				return
			}
			if !result.NoChoicesOffered {
				t.Fatalf("NoChoicesOffered = false for runtime-boundary gap %q, want true: %+v", gapCode, result)
			}
			if out.Len() != 0 {
				t.Fatalf("expected no prompt output at all for runtime-boundary gap %q, got %q", gapCode, out.String())
			}
		})
	}
}

func TestRunCompilerSetupOfferNeverOpensWhenReadinessRuntimeBlocks(t *testing.T) {
	readiness := &ReadinessResult{
		Checks: ReadinessChecks{
			Policy:         ReadinessCheck{State: ReadinessPass},
			Runtime:        ReadinessCheck{State: ReadinessFail, Code: GapNodeMissing},
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", GapTypescriptCompilerMissing, readiness, strings.NewReader("cancel\n"), &out)
	if result.RuntimeGapCode != GapNodeMissing {
		t.Fatalf("RuntimeGapCode = %q, want %q: a compiler gapCode must not open the install menu while readiness.Checks.Runtime independently fails; result=%+v", result.RuntimeGapCode, GapNodeMissing, result)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output while a runtime-boundary gap blocks compiler setup, got %q", out.String())
	}
}
