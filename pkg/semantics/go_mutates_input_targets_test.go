package semantics

import (
	"testing"
)

func TestGoMutatesInput_UpdateExpressionsMutateParameterRoots(t *testing.T) {
	tests := []goMutatesInputFindingCase{
		{
			name: "pointer selector increment",
			source: `package main

type Config struct {
	Count int
}

func f(cfg *Config) {
	cfg.Count++
}
`,
			wantName: "f:cfg",
			evidence: "cfg.Count",
		},
		{
			name: "slice index decrement",
			source: `package main

func f(items []int) {
	items[0]--
}
`,
			wantName: "f:items",
			evidence: "items[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectGoUpdateExpressionFinding(t, tt)
		})
	}
}

func expectGoUpdateExpressionFinding(t *testing.T, tt goMutatesInputFindingCase) {
	root, closeTree := mustParseGo(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeGoFeatures(root, []byte(tt.source))

	got := mutatesInputFinding(findings, tt.wantName)
	if got == nil {
		t.Fatalf("computeGoFeatures findings for update expression %q: want a mutates_input finding named %q, got %+v", tt.source, tt.wantName, findings)
	}
	if got.Evidence != tt.evidence {
		t.Errorf("mutates_input Evidence for update expression: got %q, want %q", got.Evidence, tt.evidence)
	}
}
