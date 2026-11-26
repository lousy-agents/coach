package antipatternfixtures

// Exit codes, following the census and `coach-eval patterns` (design §10).
const (
	InputInvalid   = 2
	AnalysisFailed = 3
)

// Refusal is a run the generator stops or completes only in part, with the
// exit code it gives.
type Refusal struct {
	Exit   int
	Reason string
}

func (r Refusal) Error() string { return r.Reason }

// ExitCode is the exit code the refusal gives.
func (r Refusal) ExitCode() int { return r.Exit }

func refuse(exit int, reason string) error { return Refusal{Exit: exit, Reason: reason} }
