package semantics_test

import (
	"context"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

var _ = Describe("React component orchestration density facts (epic #139 Story 1/2): component records", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
	})

	When("P1: WorkspacePage.tsx has three coordinated useState bindings, an effect, three workspace branches, imperative DOM calls, and shared panel props", func() {
		It("shall attach exactly one WorkspacePage record with the locked fact shape", func() {
			result := analyzeTSX(analyzer, "WorkspacePage.tsx", reactOrchestrationP1)

			rec, ok := reactComponentByName(result.ReactComponents, "WorkspacePage")
			Expect(ok).To(BeTrue(), "expected a react_components record named WorkspacePage, got %+v", result.ReactComponents)
			Expect(countReactComponentsNamed(result.ReactComponents, "WorkspacePage")).To(Equal(1), "expected exactly one WorkspacePage record, got %+v", result.ReactComponents)
			assertWorkspacePageShape(rec)
		})
	})

	When("P-memo: WorkspacePageMemo.tsx wraps the same component body in memo(...)", func() {
		It("shall attach one WorkspacePage record with the same fact shape as P1", func() {
			result := analyzeTSX(analyzer, "WorkspacePageMemo.tsx", reactOrchestrationPMemo)

			rec, ok := reactComponentByName(result.ReactComponents, "WorkspacePage")
			Expect(ok).To(BeTrue(), "expected a react_components record named WorkspacePage under memo(), got %+v", result.ReactComponents)
			Expect(countReactComponentsNamed(result.ReactComponents, "WorkspacePage")).To(Equal(1), "expected exactly one WorkspacePage record under memo(), got %+v", result.ReactComponents)
			assertWorkspacePageShape(rec)
		})
	})

	When("P-forwardRef: WorkspacePage is wrapped in forwardRef(...)", func() {
		It("shall attach one WorkspacePage record with the same fact shape as P1", func() {
			const src = `"use client";

import { forwardRef, useEffect, useState } from "react";

export default forwardRef(function WorkspacePage(_props, _ref) {
  const [activeView, setActiveView] = useState("list");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [filterText, setFilterText] = useState("");
  const header = document.getElementById("workspace-header");

  useEffect(() => {
    setFilterText("");
    setActiveView("list");
  }, [selectedId]);

  return (
    <div>
      {activeView === "list" ? (
        <ListPanel
          selectedId={selectedId}
          filterText={filterText}
          onSelect={setSelectedId}
        />
      ) : activeView === "detail" ? (
        <DetailPanel selectedId={selectedId} onBack={() => setActiveView("list")} />
      ) : activeView === "settings" ? (
        <SettingsPanel filterText={filterText} onFilter={setFilterText} />
      ) : null}
      <button
        type="button"
        onClick={() => {
          header?.focus();
          setActiveView("settings");
        }}
      >
        Settings
      </button>
    </div>
  );
});

function ListPanel(_props: {
  selectedId: string | null;
  filterText: string;
  onSelect: (id: string) => void;
}) {
  return <section />;
}
function DetailPanel(_props: { selectedId: string | null; onBack: () => void }) {
  return <section />;
}
function SettingsPanel(_props: { filterText: string; onFilter: (v: string) => void }) {
  return <section />;
}
`
			result := analyzeTSX(analyzer, "WorkspacePageForwardRef.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "WorkspacePage")
			Expect(ok).To(BeTrue(), "expected a react_components record named WorkspacePage under forwardRef(), got %+v", result.ReactComponents)
			Expect(countReactComponentsNamed(result.ReactComponents, "WorkspacePage")).To(Equal(1))
			assertWorkspacePageShape(rec)
		})
	})

	When("N4: the coordinating logic lives in a non-component helper function, not the exported component", func() {
		It("shall attach an empty Page record and no helperOrchestrator record", func() {
			result := analyzeTSX(analyzer, "N4Page.tsx", reactOrchestrationN4)

			_, helperFound := reactComponentByName(result.ReactComponents, "helperOrchestrator")
			Expect(helperFound).To(BeFalse(), "helperOrchestrator is not a component and must not get a react_components record")

			page, pageFound := reactComponentByName(result.ReactComponents, "Page")
			Expect(pageFound).To(BeTrue(), "expected a react_components record named Page")
			Expect(countReactComponentsNamed(result.ReactComponents, "Page")).To(Equal(1), "expected exactly one Page record, got %+v", result.ReactComponents)
			Expect(page.UseState).To(BeEmpty())
			Expect(page.CoordinatedTransitions).To(BeEmpty())
			Expect(page.WorkspaceBranches).To(BeEmpty())
			Expect(page.ImperativeUI).To(BeEmpty())
			Expect(page.SharedPanelDeps).To(BeEmpty())
		})
	})

	When("N5: ServerPage.tsx has no \"use client\" directive and no hooks", func() {
		It("shall leave react_components empty", func() {
			result := analyzeTSX(analyzer, "ServerPage.tsx", reactOrchestrationN5)
			Expect(result.ReactComponents).To(BeEmpty())
		})
	})

	When("N6: page.go is a Go file with no React constructs at all", func() {
		It("shall leave react_components empty", func() {
			result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
				Path:     "page.go",
				Language: semantics.LanguageGo,
				Content:  []byte(reactOrchestrationN6Go),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ReactComponents).To(BeEmpty())
		})
	})

	When("N9: formatDate.tsx has \"use client\" but no JSX at all", func() {
		It("shall leave react_components empty", func() {
			result := analyzeTSX(analyzer, "formatDate.tsx", reactOrchestrationN9)
			Expect(result.ReactComponents).To(BeEmpty())
		})
	})

	When("LanguageTypeScript analyzes a module with \"use client\" but no JSX", func() {
		It("shall leave react_components empty (JSX body is required for candidacy)", func() {
			const src = `"use client";

import { useState } from "react";

export function formatCount(n: number): number {
  const [x, setX] = useState(n);
  return x;
}
`
			result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
				Path:     "formatCount.ts",
				Language: semantics.LanguageTypeScript,
				Content:  []byte(src),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
			Expect(result.ReactComponents).To(BeEmpty())
		})
	})

	When("TSX source has syntax errors", func() {
		It("shall leave react_components empty on the partial result", func() {
			result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
				Path:     "broken.tsx",
				Language: semantics.LanguageTSX,
				Content:  []byte("export function Broken() { const [a, setA] = useState(1"),
			})
			Expect(result).NotTo(BeNil())
			Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("syntax_errors")))
			Expect(errors.Is(err, semantics.ErrSyntax)).To(BeTrue())
			Expect(result.ReactComponents).To(BeEmpty())
		})
	})

	When("a TSX module opens with a single-quoted 'use client' directive", func() {
		It("shall report ClientKind use_client_directive on the exported component", func() {
			const src = `'use client';

export function ClientPage() {
  return <div>Hello</div>;
}
`
			result := analyzeTSX(analyzer, "ClientPage.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "ClientPage")
			Expect(ok).To(BeTrue(), "expected a react_components record named ClientPage, got %+v", result.ReactComponents)
			Expect(rec.ClientKind).To(Equal("use_client_directive"))
		})
	})

	When("a TSX component has hooks and JSX but no \"use client\" directive", func() {
		It("shall attach a record with client_kind hooks_and_jsx", func() {
			const src = `import { useState } from "react";

export function ClientWidget() {
  const [n, setN] = useState(0);
  return <div>{n}</div>;
}
`
			result := analyzeTSX(analyzer, "ClientWidget.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "ClientWidget")
			Expect(ok).To(BeTrue(), "expected a react_components record named ClientWidget, got %+v", result.ReactComponents)
			Expect(rec.ClientKind).To(Equal("hooks_and_jsx"))
			Expect(rec.UseState).To(HaveLen(1))
			if len(rec.UseState) == 1 {
				Expect(rec.UseState[0].Binding).To(Equal("n"))
				Expect(rec.UseState[0].Setter).To(Equal("setN"))
			}
			assertNonEmptyLocation(rec.Location, "ClientWidget")
		})
	})

	When("a component is exported once via a const declaration and again via a separate export default statement", func() {
		It("shall attach exactly one Page record, not two", func() {
			const src = `"use client";

import { useState } from "react";

export const Page = () => {
  const [x] = useState(1);
  return <div>{x}</div>;
};
export default Page;
`
			result := analyzeTSX(analyzer, "Page.tsx", src)

			Expect(countReactComponentsNamed(result.ReactComponents, "Page")).To(Equal(1), "expected exactly one Page record, got %+v", result.ReactComponents)
		})
	})

	When("a component is exported once via export { Name } and again via export { Name as default }", func() {
		It("shall attach exactly one Page record, not two", func() {
			const src = `"use client";

import { useState } from "react";

function Page() {
  const [x] = useState(1);
  return <div>{x}</div>;
}
export { Page };
export { Page as default };
`
			result := analyzeTSX(analyzer, "Page.tsx", src)

			Expect(countReactComponentsNamed(result.ReactComponents, "Page")).To(Equal(1), "expected exactly one Page record, got %+v", result.ReactComponents)
		})
	})

	When("AnalyzeBytes is invoked twice on identical P1 bytes", func() {
		It("shall produce byte-identical react_components JSON", func() {
			in := semantics.FileInput{
				Path:     "WorkspacePage.tsx",
				Language: semantics.LanguageTSX,
				Content:  []byte(reactOrchestrationP1),
			}
			first, err := analyzer.AnalyzeBytes(context.Background(), in)
			Expect(err).NotTo(HaveOccurred())
			second, err := analyzer.AnalyzeBytes(context.Background(), in)
			Expect(err).NotTo(HaveOccurred())

			firstJSON, err := json.Marshal(first.ReactComponents)
			Expect(err).NotTo(HaveOccurred())
			secondJSON, err := json.Marshal(second.ReactComponents)
			Expect(err).NotTo(HaveOccurred())
			Expect(secondJSON).To(Equal(firstJSON))
		})
	})
})
