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

type argsSchemaDoc struct {
	Type       string                    `json:"type"`
	Required   []string                  `json:"required"`
	Properties map[string]argsPropSchema `json:"properties"`
	Items      *argsSchemaDoc            `json:"items"`
}

type argsPropSchema struct {
	Type  json.RawMessage `json:"type"`
	Items *argsSchemaDoc  `json:"items"`
	types []string
}

func (p *argsPropSchema) UnmarshalJSON(data []byte) error {
	type alias struct {
		Type  json.RawMessage `json:"type"`
		Items *argsSchemaDoc  `json:"items"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	p.Type = a.Type
	p.Items = a.Items
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

	if len(args) == 0 {
		return fmt.Errorf("%w: args must be a JSON object", ErrInvalidArgs)
	}
	var value any
	if err := json.Unmarshal(args, &value); err != nil {
		return fmt.Errorf("%w: args are not valid JSON", ErrInvalidArgs)
	}
	return validateAgainstSchema("", value, &sch)
}

func validateAgainstSchema(path string, value any, sch *argsSchemaDoc) error {
	if sch == nil {
		return nil
	}
	if sch.Type != "" && sch.Type != "object" && sch.Type != "array" {
		// Leaf type checks are handled by the parent property path.
		return nil
	}

	switch sch.Type {
	case "", "object":
		obj, ok := value.(map[string]any)
		if !ok {
			label := path
			if label == "" {
				label = "args"
			}
			return fmt.Errorf("%w: %s must be a JSON object", ErrInvalidArgs, label)
		}
		for _, req := range sch.Required {
			if _, present := obj[req]; !present {
				label := req
				if path != "" {
					label = path + "." + req
				}
				return fmt.Errorf("%w: missing required property %q", ErrInvalidArgs, label)
			}
		}
		for name, prop := range sch.Properties {
			raw, present := obj[name]
			if !present {
				continue
			}
			propPath := name
			if path != "" {
				propPath = path + "." + name
			}
			if err := checkPropType(propPath, raw, prop.types); err != nil {
				return err
			}
			if prop.Items != nil {
				if err := validateArrayItems(propPath, raw, prop.Items); err != nil {
					return err
				}
			}
		}
		return nil
	case "array":
		return validateArrayItems(path, value, sch.Items)
	default:
		return nil
	}
}

func validateArrayItems(path string, value any, itemSchema *argsSchemaDoc) error {
	if itemSchema == nil {
		return nil
	}
	arr, ok := value.([]any)
	if !ok {
		label := path
		if label == "" {
			label = "args"
		}
		return fmt.Errorf("%w: %s must be a JSON array", ErrInvalidArgs, label)
	}
	for i, item := range arr {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if itemSchema.Type == "object" || itemSchema.Type == "" || len(itemSchema.Required) > 0 || len(itemSchema.Properties) > 0 {
			if err := validateAgainstSchema(itemPath, item, itemSchema); err != nil {
				return err
			}
			continue
		}
		if err := checkPropType(itemPath, item, []string{itemSchema.Type}); err != nil {
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
