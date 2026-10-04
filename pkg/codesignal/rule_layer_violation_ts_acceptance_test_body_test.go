package codesignal_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
)

func body_ruleLayerViolationTsAcceptanceTest_emitsProjectChangesDeterministicallyOrderedByImp_360() {
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
			{From: "file:pkg/api/a.ts", To: "file:pkg/store/s.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/api/a.ts:1"},
			{From: "file:pkg/handlers/h.ts", To: "file:pkg/store/s.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/handlers/h.ts:1"},
			{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/handlers/h.ts:1"},
		},
	}

	changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, policy, "1", "backend-1", "digest-1")

	Expect(diagnostics).To(BeEmpty())
	Expect(changes).To(HaveLen(3))
	semanticKeys := make([]string, len(changes))
	for i, change := range changes {
		semanticKeys[i] = change.SemanticKey
	}
	Expect(semanticKeys).To(Equal([]string{
		"architecture.layer_violation:pkg/api/a.ts->pkg/store/s.ts",
		"architecture.layer_violation:pkg/handlers/h.ts->pkg/db/d.ts",
		"architecture.layer_violation:pkg/handlers/h.ts->pkg/store/s.ts",
	}))
}
