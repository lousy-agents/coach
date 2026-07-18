package codesignalcli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// AnalyzeChanges reads HEAD and merge-base content for each selected file via
// `git show`, analyzes both sides with semantics.Analyzer, computes changed
// line ranges from the unified diff between mergeBaseSHA and HEAD, and
// builds one codesignal.Report. A per-file failure (an unreadable path, a
// semantics error, an unparsable diff) is reported as a diagnostic; it never
// stops analysis of the remaining files.
func AnalyzeChanges(ctx context.Context, dir, headSHA, mergeBaseSHA string, files []SelectedFile, extraDiagnostics []codesignal.Diagnostic, appliedScope string, excluded []codesignal.CoverageGroup) (*codesignal.Report, error) {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		return nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: %s", err)}
	}

	var fileChanges []codesignal.FileChange
	diagnostics := append([]codesignal.Diagnostic(nil), extraDiagnostics...)

	for _, sf := range files {
		var fc *codesignal.FileChange
		var fileDiagnostics []codesignal.Diagnostic
		if sf.Status == "removed" {
			fc, fileDiagnostics = analyzeRemovedFile(ctx, analyzer, dir, mergeBaseSHA, sf)
		} else {
			fc, fileDiagnostics = analyzeAddedOrModifiedFile(ctx, analyzer, dir, headSHA, mergeBaseSHA, sf)
		}
		diagnostics = append(diagnostics, fileDiagnostics...)
		if fc != nil {
			fileChanges = append(fileChanges, *fc)
		}
	}

	builder, err := codesignal.New(codesignal.Options{IncludeResolved: true})
	if err != nil {
		return nil, err
	}

	// Only Excluded is meaningful on a diff-flow report; TrackedFilesDiscovered/
	// FilesAnalyzed/FilesUnanalyzable are baseline-only accounting fields and
	// are deliberately left zero here (see codesignal.Coverage's doc comment).
	var coverage *codesignal.Coverage
	if len(excluded) > 0 {
		coverage = &codesignal.Coverage{Excluded: excluded}
	}

	return builder.Build(ctx, codesignal.Input{
		Scope:       codesignal.Scope{Repository: "", Revision: headSHA, Base: mergeBaseSHA, AppliedScope: appliedScope},
		Files:       fileChanges,
		Diagnostics: diagnostics,
		Coverage:    coverage,
	})
}

// AnalyzeBaseline reads revisionSHA content for each selected file via a
// single long-lived `git cat-file --batch` process (see
// revisionFileReader) rather than one `git show` subprocess per file, and
// analyzes each file's content with semantics.Analyzer. Unlike AnalyzeChanges,
// there is no base content and no changed-ranges computation: a Repository
// Baseline is not a comparison against anything, just every tracked file at
// one revision. A per-file failure (an unreadable path, a semantics error)
// is reported as a diagnostic and tallied into coverage.FilesUnanalyzable;
// it never stops analysis of the remaining files. A file that parses,
// including one whose ParseStatus is "syntax_errors", still gets a
// codesignal.FileChange so Build's existing processHeadResult emits its own
// syntax_errors diagnostic -- callers must not duplicate that here.
func AnalyzeBaseline(ctx context.Context, dir, revisionSHA string, files []SelectedFile, extraDiagnostics []codesignal.Diagnostic, coverage codesignal.Coverage) (*codesignal.Report, error) {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		return nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: %s", err)}
	}

	reader, err := newRevisionFileReader(dir, revisionSHA)
	if err != nil {
		return nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: starting git cat-file --batch failed: %s", err)}
	}
	defer func() { _ = reader.close() }()

	var fileChanges []codesignal.FileChange
	diagnostics := append([]codesignal.Diagnostic(nil), extraDiagnostics...)

	for _, sf := range files {
		headBytes, err := reader.next(sf.Path)
		if err != nil {
			diagnostics = append(diagnostics, codesignal.Diagnostic{
				Path:    sf.Path,
				Kind:    "head_read_failed",
				Message: fmt.Sprintf("reading head content for %q: %s", sf.Path, err),
			})
			coverage.FilesUnanalyzable++
			continue
		}

		headResult, headErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: headBytes})
		if headErr != nil && !errors.Is(headErr, semantics.ErrSyntax) {
			diagnostics = append(diagnostics, mapSemanticsError(sf.Path, headErr))
			coverage.FilesUnanalyzable++
			continue
		}

		fileChanges = append(fileChanges, codesignal.FileChange{Path: sf.Path, SourceScope: sf.SourceScope, Head: headResult})
		if headResult.ParseStatus == "ok" {
			coverage.FilesAnalyzed++
		} else {
			coverage.FilesUnanalyzable++
		}
	}

	builder, err := codesignal.New(codesignal.Options{Baseline: true})
	if err != nil {
		return nil, err
	}

	return builder.Build(ctx, codesignal.Input{
		Scope:       codesignal.Scope{Revision: revisionSHA},
		Files:       fileChanges,
		Diagnostics: diagnostics,
		Coverage:    &coverage,
	})
}

// analyzeAddedOrModifiedFile handles "added" and "modified" SelectedFiles:
// HEAD content is mandatory, base content is read (and analyzed) only when
// the file already existed at mergeBaseSHA.
func analyzeAddedOrModifiedFile(ctx context.Context, analyzer *semantics.Analyzer, dir, headSHA, mergeBaseSHA string, sf SelectedFile) (*codesignal.FileChange, []codesignal.Diagnostic) {
	headBytes, err := runGitBytes(dir, "show", headSHA+":"+sf.Path)
	if err != nil {
		return nil, []codesignal.Diagnostic{{
			Path:    sf.Path,
			Kind:    "head_read_failed",
			Message: fmt.Sprintf("reading head content for %q: %s", sf.Path, err),
		}}
	}

	headResult, headErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: headBytes})
	if headErr != nil && !errors.Is(headErr, semantics.ErrSyntax) {
		return nil, []codesignal.Diagnostic{mapSemanticsError(sf.Path, headErr)}
	}

	fc := codesignal.FileChange{Path: sf.Path, Status: sf.Status, SourceScope: sf.SourceScope, Head: headResult}
	var diagnostics []codesignal.Diagnostic

	if sf.Status == "modified" {
		baseBytes, err := runGitBytes(dir, "show", mergeBaseSHA+":"+sf.Path)
		if err != nil {
			// A "modified" status means Git already knows this path existed at
			// mergeBaseSHA, so a failed read here is a real problem (e.g. a
			// corrupted object), not an added-file's expected absence -- it
			// must be surfaced, not silently treated as "no base".
			diagnostics = append(diagnostics, codesignal.Diagnostic{
				Path:    sf.Path,
				Kind:    "base_read_failed",
				Message: fmt.Sprintf("reading base content for %q: %s", sf.Path, err),
			})
		} else {
			baseResult, baseErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: baseBytes})
			switch {
			case baseErr == nil:
				fc.Base = baseResult
			case errors.Is(baseErr, semantics.ErrSyntax):
				diagnostics = append(diagnostics, baseSyntaxDiagnostics(sf.Path, baseErr)...)
			default:
				diagnostics = append(diagnostics, codesignal.Diagnostic{
					Path:    sf.Path,
					Kind:    "base_analysis_failed",
					Message: baseErr.Error(),
				})
			}
		}
	}

	ranges, rangeDiagnostic := computeChangedRanges(dir, mergeBaseSHA, sf.Path)
	if rangeDiagnostic != nil {
		diagnostics = append(diagnostics, *rangeDiagnostic)
	} else {
		fc.ChangedRanges = ranges
	}

	return &fc, diagnostics
}

// analyzeRemovedFile handles "removed" SelectedFiles: only base content
// exists, and there is no changed-range computation (no HEAD content to
// place ranges against).
func analyzeRemovedFile(ctx context.Context, analyzer *semantics.Analyzer, dir, mergeBaseSHA string, sf SelectedFile) (*codesignal.FileChange, []codesignal.Diagnostic) {
	baseBytes, err := runGitBytes(dir, "show", mergeBaseSHA+":"+sf.Path)
	if err != nil {
		return nil, []codesignal.Diagnostic{{
			Path:    sf.Path,
			Kind:    "base_read_failed",
			Message: fmt.Sprintf("reading base content for %q: %s", sf.Path, err),
		}}
	}

	baseResult, baseErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: baseBytes})
	switch {
	case baseErr == nil:
		return &codesignal.FileChange{Path: sf.Path, Status: sf.Status, SourceScope: sf.SourceScope, Base: baseResult}, nil
	case errors.Is(baseErr, semantics.ErrSyntax):
		return nil, baseSyntaxDiagnostics(sf.Path, baseErr)
	default:
		return nil, []codesignal.Diagnostic{mapSemanticsError(sf.Path, baseErr)}
	}
}

// baseSyntaxDiagnostics emits one "base_syntax_errors" diagnostic per syntax
// issue found in a base analysis, using errors.As to recover the specific
// issues from baseErr.
func baseSyntaxDiagnostics(path string, baseErr error) []codesignal.Diagnostic {
	var syntaxErr *semantics.SyntaxError
	if !errors.As(baseErr, &syntaxErr) {
		return []codesignal.Diagnostic{{
			Path:    path,
			Kind:    "base_syntax_errors",
			Message: baseErr.Error(),
		}}
	}

	diagnostics := make([]codesignal.Diagnostic, 0, len(syntaxErr.Issues))
	for _, issue := range syntaxErr.Issues {
		location := issue.Location
		diagnostics = append(diagnostics, codesignal.Diagnostic{
			Path:     path,
			Kind:     "base_syntax_errors",
			Location: &location,
			Message:  fmt.Sprintf("base analysis found a syntax issue of kind %q", issue.Kind),
		})
	}
	return diagnostics
}

func mapSemanticsError(path string, err error) codesignal.Diagnostic {
	kind := "analysis_failed"
	switch {
	case errors.Is(err, semantics.ErrEmptyContent):
		kind = "empty_content"
	case errors.Is(err, semantics.ErrBinaryContent):
		kind = "binary_content"
	case errors.Is(err, semantics.ErrFileTooLarge):
		kind = "file_too_large"
	case errors.Is(err, semantics.ErrUnsupportedLanguage):
		kind = "unsupported_language"
	}
	return codesignal.Diagnostic{Path: path, Kind: kind, Message: err.Error()}
}

// computeChangedRanges runs `git diff --unified=0` for path between
// mergeBaseSHA and HEAD and parses the resulting hunk headers into
// changed-line ranges. Any failure (to run git, or to parse its output) is
// reported as a single "diff_analysis_failed" diagnostic with no ranges.
func computeChangedRanges(dir, mergeBaseSHA, path string) ([]codesignal.LineRange, *codesignal.Diagnostic) {
	output, err := runGitBytes(dir, "diff", "--unified=0", "--no-ext-diff", mergeBaseSHA, "HEAD", "--", path)
	if err != nil {
		return nil, &codesignal.Diagnostic{
			Path:    path,
			Kind:    "diff_analysis_failed",
			Message: fmt.Sprintf("computing changed ranges for %q: %s", path, err),
		}
	}

	ranges, err := parseChangedRanges(output)
	if err != nil {
		return nil, &codesignal.Diagnostic{
			Path:    path,
			Kind:    "diff_analysis_failed",
			Message: fmt.Sprintf("parsing diff for %q: %s", path, err),
		}
	}
	return ranges, nil
}

// hunkHeaderPattern matches a unified diff hunk header:
// "@@ -oldStart[,oldCount] +newStart[,newCount] @@" (with an optional
// trailing section heading, which is ignored). Omitted counts mean 1.
var hunkHeaderPattern = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// parseChangedRanges parses ONLY unified diff hunk headers out of diff,
// ignoring every other line (file headers, context, +/- lines). Each hunk
// with a non-zero new-side line count becomes one 0-based inclusive
// codesignal.LineRange; pure-deletion hunks (new count 0) contribute no
// range.
func parseChangedRanges(diff []byte) ([]codesignal.LineRange, error) {
	var ranges []codesignal.LineRange

	scanner := bufio.NewScanner(bytes.NewReader(diff))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}

		match := hunkHeaderPattern.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("unparsable hunk header: %q", line)
		}

		newStart, err := strconv.Atoi(match[1])
		if err != nil {
			return nil, fmt.Errorf("invalid hunk new-start in %q: %w", line, err)
		}

		newCount := 1
		if match[2] != "" {
			newCount, err = strconv.Atoi(match[2])
			if err != nil {
				return nil, fmt.Errorf("invalid hunk new-count in %q: %w", line, err)
			}
		}

		if newCount == 0 {
			continue
		}

		ranges = append(ranges, codesignal.LineRange{
			StartRow: uint(newStart - 1),
			EndRow:   uint(newStart + newCount - 2),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ranges, nil
}
