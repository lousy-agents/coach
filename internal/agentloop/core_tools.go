package agentloop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func coreSemanticsSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"required":["path","language","content"],
		"properties":{
			"path":{"type":"string"},
			"language":{"type":"string"},
			"content":{"type":"string"}
		}
	}`)
}

func coreCodeSignalSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"required":["files"],
		"properties":{
			"files":{
				"type":"array",
				"items":{
					"type":"object",
					"required":["path"],
					"properties":{
						"path":{"type":"string"}
					}
				}
			},
			"baseline":{"type":"boolean"},
			"repository":{"type":"string"},
			"revision":{"type":"string"},
			"include_resolved":{"type":"boolean"},
			"diagnostics":{"type":"array"}
		}
	}`)
}

// DefaultSemanticsAnalyze returns a thin wrapper around pkg/semantics.
func DefaultSemanticsAnalyze() ToolHandler {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		return func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return nil, fmt.Errorf("agentloop: default semantics analyzer: %w", err)
		}
	}
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		var in struct {
			Path     string `json:"path"`
			Language string `json:"language"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidArgs, err)
		}
		res, analyzeErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{
			Path:     in.Path,
			Language: semantics.Language(in.Language),
			Content:  []byte(in.Content),
		})
		if res == nil {
			return nil, analyzeErr
		}
		out, err := json.Marshal(res)
		if err != nil {
			return nil, err
		}
		// Syntax errors yield a partial Result with a non-nil error; surface both.
		return out, analyzeErr
	}
}

// DefaultCodeSignalReport returns a thin wrapper around pkg/codesignal.
func DefaultCodeSignalReport() ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		var in struct {
			Files           []codesignal.FileChange `json:"files"`
			Baseline        bool                    `json:"baseline"`
			Repository      string                  `json:"repository"`
			Revision        string                  `json:"revision"`
			IncludeResolved bool                    `json:"include_resolved"`
			Diagnostics     []codesignal.Diagnostic `json:"diagnostics"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidArgs, err)
		}
		b, err := codesignal.New(codesignal.Options{
			Baseline:        in.Baseline,
			IncludeResolved: in.IncludeResolved,
		})
		if err != nil {
			return nil, err
		}
		report, err := b.Build(ctx, codesignal.Input{
			Scope: codesignal.Scope{
				Repository: in.Repository,
				Revision:   in.Revision,
				Baseline:   in.Baseline,
			},
			Files:       in.Files,
			Diagnostics: in.Diagnostics,
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(report)
	}
}
