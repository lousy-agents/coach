package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
)

// Core tool names always registered on a Loop (baseline v1).
// Additional always-on tools are added to coreToolDefs — not scattered through New/Register.
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

// coreToolDef is one always-registered tool. Extend this table when adding
// package-default tools (PR-history GitHub tools stay job-specific via Register).
type coreToolDef struct {
	name   string
	schema func() json.RawMessage
	pick   func(Options) ToolHandler
}

func coreToolDefs() []coreToolDef {
	return []coreToolDef{
		{
			name:   ToolSemanticsAnalyze,
			schema: coreSemanticsSchema,
			pick: func(o Options) ToolHandler {
				if o.SemanticsAnalyze != nil {
					return o.SemanticsAnalyze
				}
				return DefaultSemanticsAnalyze()
			},
		},
		{
			name:   ToolCodeSignalReport,
			schema: coreCodeSignalSchema,
			pick: func(o Options) ToolHandler {
				if o.CodeSignalReport != nil {
					return o.CodeSignalReport
				}
				return DefaultCodeSignalReport()
			},
		},
	}
}

func isCoreToolName(name string) bool {
	for _, def := range coreToolDefs() {
		if def.name == name {
			return true
		}
	}
	return false
}

func registerCoreTools(tools map[string]registeredTool, opts Options) {
	for _, def := range coreToolDefs() {
		tools[def.name] = registeredTool{
			schema:  def.schema(),
			handler: def.pick(opts),
		}
	}
}

func rejectCoreToolRegister(name string) error {
	if !isCoreToolName(name) {
		return nil
	}
	return fmt.Errorf("agentloop: cannot replace core tool %q via Register; inject via Options", name)
}
