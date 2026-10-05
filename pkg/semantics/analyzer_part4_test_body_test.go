package semantics

import (
	"context"

	"errors"

	"testing"
)

type rejectedInputCase struct {
	name    string
	in      FileInput
	wantErr error
}

func expectAnalyzeBytesRejects(t *testing.T, a *Analyzer, tt rejectedInputCase) {
	result, err := a.AnalyzeBytes(context.Background(), tt.in)

	if result != nil {
		t.Errorf("AnalyzeBytes(%+v): got non-nil result %+v, want nil", tt.in, result)
	}
	if !errors.Is(err, tt.wantErr) {
		t.Errorf("AnalyzeBytes(%+v): got err %v, want errors.Is(err, %v) to hold", tt.in, err, tt.wantErr)
	}
}
