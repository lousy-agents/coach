package agentloop

import "errors"

var (
	// ErrUnknownTool is returned when a call targets a name with no registered handler.
	ErrUnknownTool = errors.New("agentloop: unknown tool")

	// ErrBudgetExceeded is returned when a run exhausts max_tool_calls, max_model_calls,
	// or max_wall_time.
	ErrBudgetExceeded = errors.New("agentloop: budget exceeded")

	// ErrInvalidArgs is returned when tool arguments fail the registered args schema.
	ErrInvalidArgs = errors.New("agentloop: invalid tool arguments")
)
