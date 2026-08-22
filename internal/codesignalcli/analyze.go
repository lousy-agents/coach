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

func AnalyzeChanges(ctx context.Context, dir, headSHA, mergeBaseSHA string, files []SelectedFile, extraDiagnostics []codesignal.Diagnostic, appliedScope string, excluded []codesignal.CoverageGroup, project *ProjectAnalysis) (*codesignal.Report, error) {
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

	var coverage *codesignal.Coverage
	if len(excluded) > 0 {
		coverage = &codesignal.Coverage{Excluded: excluded}
	}

	opts := codesignal.Options{IncludeResolved: true}
	input := codesignal.Input{
		Scope:       codesignal.Scope{Repository: "", Revision: headSHA, Base: mergeBaseSHA, AppliedScope: appliedScope},
		Files:       fileChanges,
		Diagnostics: diagnostics,
		Coverage:    coverage,
	}
	input, opts, err = applyProjectBackend(ctx, input, opts, project, dir, headSHA, mergeBaseSHA, false)
	if err != nil {
		return nil, err
	}
	builder, err := codesignal.New(opts)
	if err != nil {
		return nil, err
	}
	return builder.Build(ctx, input)
}

// AnalyzeBaseline uses a single long-lived `git cat-file --batch` process
// (revisionFileReader) instead of one `git show` subprocess per file.
func AnalyzeBaseline(ctx context.Context, dir, revisionSHA string, files []SelectedFile, extraDiagnostics []codesignal.Diagnostic, appliedScope string, coverage codesignal.Coverage, project *ProjectAnalysis) (*codesignal.Report, error) {
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

	opts := codesignal.Options{Baseline: true}
	input := codesignal.Input{
		Scope:       codesignal.Scope{Revision: revisionSHA, AppliedScope: appliedScope},
		Files:       fileChanges,
		Diagnostics: diagnostics,
		Coverage:    &coverage,
	}
	input, opts, err = applyProjectBackend(ctx, input, opts, project, dir, revisionSHA, "", true)
	if err != nil {
		return nil, err
	}
	builder, err := codesignal.New(opts)
	if err != nil {
		return nil, err
	}
	return builder.Build(ctx, input)
}

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
// "@@ -oldStart[,oldCount] +newStart[,newCount] @@" (trailing section
// heading ignored).
var hunkHeaderPattern = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// parseChangedRanges returns 0-based, inclusive codesignal.LineRange values
// derived from each hunk's new-side start/count.
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
