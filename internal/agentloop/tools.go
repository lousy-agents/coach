package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// Core tool names always registered on a Loop (baseline v1).
const (
	ToolSemanticsAnalyze = "semantics_analyze"
	ToolCodeSignalReport = "codesignal_report"
)

// ToolHandler is the function shape registered under a tool name.
type ToolHandler func(ctx context.Context, args json.RawMessage) (json.RawMessage, error)

// ToolSpec describes a named, schema-validated tool handler.
type ToolSpec struct {
	Name       string
	ArgsSchema json.RawMessage
	Handler    ToolHandler
}

type registeredTool struct {
	schema  json.RawMessage
	handler ToolHandler
}

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
			"files":{"type":"array"},
			"baseline":{"type":"boolean"},
			"repository":{"type":"string"},
			"revision":{"type":"string"},
			"include_resolved":{"type":"boolean"}
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

type argsSchemaDoc struct {
	Type       string                    `json:"type"`
	Required   []string                  `json:"required"`
	Properties map[string]argsPropSchema `json:"properties"`
}

type argsPropSchema struct {
	Type  json.RawMessage `json:"type"`
	types []string
}

func (p *argsPropSchema) UnmarshalJSON(data []byte) error {
	type alias struct {
		Type json.RawMessage `json:"type"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	p.Type = a.Type
	types, err := parseJSONTypes(p.Type)
	if err != nil {
		return err
	}
	p.types = types
	return nil
}

func parseJSONTypes(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var multi []string
	if err := json.Unmarshal(raw, &multi); err != nil {
		return nil, err
	}
	return multi, nil
}

func validateToolArgs(schema json.RawMessage, args json.RawMessage) error {
	if len(schema) == 0 {
		if len(args) == 0 {
			return nil
		}
		var v any
		if err := json.Unmarshal(args, &v); err != nil {
			return fmt.Errorf("%w: args are not valid JSON", ErrInvalidArgs)
		}
		return nil
	}

	var sch argsSchemaDoc
	if err := json.Unmarshal(schema, &sch); err != nil {
		return fmt.Errorf("%w: tool schema is not valid JSON", ErrInvalidArgs)
	}
	if sch.Type != "" && sch.Type != "object" {
		return fmt.Errorf("%w: tool schema type must be object", ErrInvalidArgs)
	}

	if len(args) == 0 {
		return fmt.Errorf("%w: args must be a JSON object", ErrInvalidArgs)
	}
	var value any
	if err := json.Unmarshal(args, &value); err != nil {
		return fmt.Errorf("%w: args are not valid JSON", ErrInvalidArgs)
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: args must be a JSON object", ErrInvalidArgs)
	}

	for _, req := range sch.Required {
		if _, present := obj[req]; !present {
			return fmt.Errorf("%w: missing required property %q", ErrInvalidArgs, req)
		}
	}

	for name, prop := range sch.Properties {
		raw, present := obj[name]
		if !present {
			continue
		}
		if err := checkPropType(name, raw, prop.types); err != nil {
			return err
		}
	}
	return nil
}

func checkPropType(name string, value any, types []string) error {
	if len(types) == 0 {
		return nil
	}
	actual := jsonTypeOf(value)
	for _, t := range types {
		if strings.EqualFold(t, actual) {
			return nil
		}
		// JSON numbers decode as float64; accept "integer" for whole numbers.
		if strings.EqualFold(t, "integer") && actual == "number" {
			if f, ok := value.(float64); ok && f == float64(int64(f)) {
				return nil
			}
		}
	}
	return fmt.Errorf("%w: property %q has type %s, expected %s", ErrInvalidArgs, name, actual, strings.Join(types, "|"))
}

func jsonTypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func cloneRawMessage(m json.RawMessage) json.RawMessage {
	if m == nil {
		return nil
	}
	out := make(json.RawMessage, len(m))
	copy(out, m)
	return out
}
