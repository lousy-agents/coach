package semantics

import "testing"

type sigbodyresultPart2Test65S121892662 struct {
	r Result
	t *testing.
		T
}

func (sigRecv *sigbodyresultPart2Test65S121892662) call() {
	if sigRecv.r.ParseStatus != ParseStatus("ok") {
		sigRecv.t.
			Errorf("AC-4.4: golden react_components Result.ParseStatus: got %q, want %q", sigRecv.r.ParseStatus, "ok")
	}
}
