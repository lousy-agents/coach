package projectmodel

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/lousy-agents/coach/pkg/semantics"
	"golang.org/x/mod/modfile"
)

func analyzeGoSources(
	snapshot fs.FS,
	analyzer *semantics.Analyzer,
	allFiles []string,
	modules map[string]*modfile.File,
	packageFiles map[string][]string,
	fileModule map[string]string,
	budgets GoBudgets,
) (files []File, edges []ImportEdge, unresolvedEdges, excludedEdges int, truncated bool, diagnostics []Diagnostic) {
	analysis := goSourceAnalysis{files: make([]File, 0, len(allFiles))}
	for _, f := range allFiles {
		if analysis.consume(snapshot, analyzer, f, modules, packageFiles, fileModule, budgets) {
			break
		}
	}
	return analysis.files, analysis.edges, analysis.unresolvedEdges, analysis.excludedEdges, analysis.truncated, analysis.diagnostics
}

type goSourceAnalysis struct {
	files           []File
	edges           []ImportEdge
	unresolvedEdges int
	excludedEdges   int
	truncated       bool
	diagnostics     []Diagnostic
	filesProcessed  int
	bytesProcessed  int64
}

func (a *goSourceAnalysis) consume(
	snapshot fs.FS,
	analyzer *semantics.Analyzer,
	f string,
	modules map[string]*modfile.File,
	packageFiles map[string][]string,
	fileModule map[string]string,
	budgets GoBudgets,
) (stop bool) {
	if budgets.MaxInputFiles > 0 && a.filesProcessed >= budgets.MaxInputFiles {
		a.truncated = true
		return true
	}
	content, readErr := fs.ReadFile(snapshot, f)
	if readErr != nil {
		a.files = append(a.files, File{ID: "file:" + f, Path: f, Language: "go"})
		a.diagnostics = append(a.diagnostics, Diagnostic{Code: DiagFileUnavailable, Path: f, Message: readErr.Error()})
		a.filesProcessed++
		return false
	}
	if budgets.MaxInputBytes > 0 && a.bytesProcessed+int64(len(content)) > budgets.MaxInputBytes {
		a.truncated = true
		return true
	}
	a.bytesProcessed += int64(len(content))
	a.files = append(a.files, File{ID: "file:" + f, Path: f, Language: "go"})
	a.filesProcessed++

	result, analyzeErr := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     f,
		Language: semantics.LanguageGo,
		Content:  content,
	})
	if analyzeErr != nil {
		code := DiagFileUnavailable
		if errors.Is(analyzeErr, semantics.ErrSyntax) {
			code = DiagFileSyntaxError
		}
		a.diagnostics = append(a.diagnostics, Diagnostic{Code: code, Path: f, Message: analyzeErr.Error()})
		return false
	}

	fromID := "package:" + path.Dir(f)
	owner := modules[fileModule[f]]
	for _, imp := range result.Imports {
		kind, to := classifyGoImport(imp.Path, owner, modules, packageFiles)
		switch kind {
		case "unresolved":
			a.unresolvedEdges++
		case "excluded":
			a.excludedEdges++
		}
		a.edges = append(a.edges, ImportEdge{
			From: fromID,
			To:   to,
			Kind: kind,
			Site: fmt.Sprintf("%s:%d", f, imp.Location.StartRow+1),
		})
	}
	return false
}
