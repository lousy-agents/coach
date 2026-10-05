package semantics

import (
	"context"

	"errors"

	"testing"
)

func body_analyzerPart4Test_43(t *testing.T, a *Analyzer, tt struct {
	name    string
	in      FileInput
	wantErr error
}) {
	result, err := a.AnalyzeBytes(context.Background(), tt.in)

	if result != nil {
		t.Errorf("AnalyzeBytes(%+v): got non-nil result %+v, want nil", tt.in, result)
	}
	if !errors.Is(err, tt.wantErr) {
		t.Errorf("AnalyzeBytes(%+v): got err %v, want errors.Is(err, %v) to hold", tt.in, err, tt.wantErr)
	}
}

func body_analyzerPart4Test_AC16ContentOverMaxFileBytes_55(t *testing.T) {
	small, err := NewAnalyzer(AnalyzerOptions{MaxFileBytes: 4})
	if err != nil {
		t.Fatalf("NewAnalyzer(MaxFileBytes: 4): got err %v, want nil", err)
	}

	in := FileInput{Language: LanguageGo, Content: []byte("package main\n")}
	result, err := small.AnalyzeBytes(context.Background(), in)

	if result != nil {
		t.Errorf("AnalyzeBytes(%+v) with MaxFileBytes=4: got non-nil result %+v, want nil", in, result)
	}
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("AnalyzeBytes(%+v) with MaxFileBytes=4: got err %v, want errors.Is(err, ErrFileTooLarge) to hold", in, err)
	}
}
