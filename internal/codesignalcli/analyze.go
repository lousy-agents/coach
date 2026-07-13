package codesignalcli

import (
	"context"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// AnalyzeChanges is not yet implemented.
func AnalyzeChanges(ctx context.Context, dir, headSHA, mergeBaseSHA string, files []SelectedFile, extraDiagnostics []codesignal.Diagnostic) (*codesignal.Report, error) {
	return &codesignal.Report{SchemaVersion: "1"}, nil
}

func parseChangedRanges(diff []byte) ([]codesignal.LineRange, error) {
	return nil, nil
}

func mapSemanticsError(path string, err error) codesignal.Diagnostic {
	return codesignal.Diagnostic{Path: path, Kind: "todo", Message: err.Error()}
}
