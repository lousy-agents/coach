package projectconfig

import "testing"

func TestCauseWordedPlacesTheCauseBeforeTheFinalPeriodOfAFinishedSentence(t *testing.T) {
	refusal := &DiskPolicyRefusal{Path: "cfg/p.json", Reason: refusalLink}
	cause := string(refusal.Cause())
	if cause == "" {
		t.Fatal("fixture: a link has a cause")
	}
	tests := []struct {
		name, message, want string
	}{
		{"a message that ends with a period", "the policy is a link. restore it.", "the policy is a link. restore it (cause=" + cause + ")."},
		{"a message that does not end with a period", "the policy is a link", "the policy is a link (cause=" + cause + ")"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectWorded(t, refusal, tt.message, tt.want) })
	}
}

func expectWorded(t *testing.T, refusal *DiskPolicyRefusal, message, want string) {
	t.Helper()
	got := refusal.CauseWorded(&ConfigError{Kind: KindInvalid, Message: message})
	if worded := configErrorOf(t, got).Message; worded != want {
		t.Fatalf("message = %q, want %q", worded, want)
	}
}
