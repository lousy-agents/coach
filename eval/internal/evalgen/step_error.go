package evalgen

// Validation steps that can reject a program. The graders in the harness reject at their own steps.
const (
	StepNative         = "native"
	StepDeleteBaseline = "delete_baseline"
	StepLayerGraph     = "layer_graph"
	StepGocognit       = "gocognit"
)

// ValidationSteps lists every step in the order a program meets them.
var ValidationSteps = []string{StepNative, StepDeleteBaseline, StepLayerGraph, StepGocognit}

// StepError is a program rejected by one validation step, as distinct from a tool that could not run.
type StepError struct {
	Step string
	Err  error
}

func (e *StepError) Error() string { return e.Err.Error() }

func (e *StepError) Unwrap() error { return e.Err }
