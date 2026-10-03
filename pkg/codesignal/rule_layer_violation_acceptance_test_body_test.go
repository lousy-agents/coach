package codesignal_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
)

func body_ruleLayerViolationAcceptanceTest_emitsProjectChangesDeterministicallyOrderedByImp_137() {
	policy := codesignal.LayerPolicy{
		Layers: []codesignal.ArchitectureLayer{
			{Name: "handlers", Prefixes: []string{"pkg/handlers", "pkg/api"}},
			{Name: "db", Prefixes: []string{"pkg/db", "pkg/store"}},
		},
		ForbiddenImports: []codesignal.ForbiddenLayerImport{
			{From: "handlers", To: "db"},
		},
	}
	model := domain.Model{
		ImportEdges: []domain.ImportEdge{
			{From: "package:pkg/api", To: "package:pkg/store", Kind: "internal", Site: "pkg/api/a.go:1"},
			{From: "package:pkg/handlers", To: "package:pkg/store", Kind: "internal", Site: "pkg/handlers/h.go:2"},
			{From: "package:pkg/handlers", To: "package:pkg/db", Kind: "internal", Site: "pkg/handlers/h.go:3"},
		},
	}

	changes, diagnostics := codesignal.EvaluateGoLayerViolations(model, policy, "1", "backend-1", "digest-1")

	Expect(diagnostics).To(BeEmpty())
	Expect(changes).To(HaveLen(3))
	semanticKeys := make([]string, len(changes))
	for i, change := range changes {
		semanticKeys[i] = change.SemanticKey
	}
	Expect(semanticKeys).To(Equal([]string{
		"architecture.layer_violation:pkg/api->pkg/store",
		"architecture.layer_violation:pkg/handlers->pkg/db",
		"architecture.layer_violation:pkg/handlers->pkg/store",
	}))
}
