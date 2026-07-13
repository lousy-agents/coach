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
func AnalyzeChanges(ctx context.Context, dir, headSHA, mergeBaseSHA string, files []SelectedFile, extraDiagnostics []codesignal.Diagnostic) (*codesignal.Report, error) {
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

	return builder.Build(ctx, codesignal.Input{
		Scope:       codesignal.Scope{Repository: "", Revision: headSHA, Base: mergeBaseSHA},
		Files:       fileChanges,
		Diagnostics: diagnostics,
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

	fc := codesignal.FileChange{Path: sf.Path, Status: sf.Status, Head: headResult}
	var diagnostics []codesignal.Diagnostic

	if sf.Status == "modified" {
		if baseBytes, ok := readBaseBytes(dir, mergeBaseSHA, sf.Path); ok {
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
		return &codesignal.FileChange{Path: sf.Path, Status: sf.Status, Base: baseResult}, nil
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

// mapSemanticsError maps a non-ErrSyntax semantics error to its CLI
// diagnostic kind.
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

// readBaseBytes reads path at mergeBaseSHA. A failure is treated as "the
// path did not exist at that revision" (e.g. an added file) rather than as
// an operational error: ok is false and content is nil.
func readBaseBytes(dir, mergeBaseSHA, path string) ([]byte, bool) {
	content, err := runGitBytes(dir, "show", mergeBaseSHA+":"+path)
	if err != nil {
		return nil, false
	}
	return content, true
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
