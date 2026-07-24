package rubrics

import (
	"context"
	"testing"
)

func TestRun_EmptyDefinitionIDBeforeNilGateway(t *testing.T) {
	r, err := Run(context.Background(), nil, Definition{}, nil)
	if err != nil {
		t.Fatalf("Run: unexpected err %v", err)
	}
	if r.Diagnostic == nil {
		t.Fatal("expected diagnostic")
	}
	if r.Diagnostic.Scope != "rubric:unknown" {
		t.Fatalf("scope: got %q, want rubric:unknown", r.Diagnostic.Scope)
	}
}
