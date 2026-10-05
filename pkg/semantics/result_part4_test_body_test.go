package semantics

import (
	"encoding/json"

	"testing"
)

func body_resultPart4Test_50(t *testing.T, tt struct {
	name   string
	status ParseStatus
	want   string
}) {
	raw, err := json.Marshal(tt.status)
	if err != nil {
		t.Fatalf("AC-4.3: marshaling ParseStatus %q must not fail: %v", tt.status, err)
	}
	if string(raw) != tt.want {
		t.Errorf("AC-4.3: ParseStatus %q marshaled: got %s, want %s", tt.status, string(raw), tt.want)
	}
}

func body_resultPart4Test_108(t *testing.T, tt struct {
	name   string
	issues []SyntaxIssue
	want   string
}) {
	err := &SyntaxError{Issues: tt.issues}
	if got := err.Error(); got != tt.want {
		t.Errorf("AC-2.3: SyntaxError.Error() for %d issue(s): got %q, want %q", len(tt.issues), got, tt.want)
	}
}
