package semantics_test

import (
	"context"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func reactComponentByName(records []semantics.ReactComponentFacts, name string) (semantics.ReactComponentFacts, bool) {
	for _, r := range records {
		if r.Name == name {
			return r, true
		}
	}
	return semantics.ReactComponentFacts{}, false
}

func countReactComponentsNamed(records []semantics.ReactComponentFacts, name string) int {
	count := 0
	for _, r := range records {
		if r.Name == name {
			count++
		}
	}
	return count
}

func analyzeTSX(analyzer *semantics.Analyzer, path, source string) *semantics.Result {
	GinkgoHelper()
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     path,
		Language: semantics.LanguageTSX,
		Content:  []byte(source),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result).NotTo(BeNil())
	Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
	return result
}

const reactOrchestrationP1 = `"use client";

import { useEffect, useState } from "react";

export function WorkspacePage() {
  const [activeView, setActiveView] = useState("list");
  const [selectedId, setSelectedId] = useState(null);
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
}

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

const reactOrchestrationN4 = `"use client";

import { useEffect, useState } from "react";

function helperOrchestrator() {
  const [activeView, setActiveView] = useState("a");
  const [selectedId, setSelectedId] = useState("x");
  const [filterText, setFilterText] = useState("");
  useEffect(() => {
    setFilterText("");
    setActiveView("a");
  }, [selectedId]);
  return activeView === "a" ? (
    <A />
  ) : activeView === "b" ? (
    <B />
  ) : activeView === "c" ? (
    <C />
  ) : null;
}

export function Page() {
  return <div>{helperOrchestrator()}</div>;
}

function A() {
  return <section />;
}
function B() {
  return <section />;
}
function C() {
  return <section />;
}
`

const reactOrchestrationN5 = `export function ServerPage(props: { children: string }) {
  return <div>{props.children}</div>;
}
`

const reactOrchestrationN6Go = `package p

func WorkspacePage() { println("useState") }
`

const reactOrchestrationN9 = `"use client";

export function formatDate(d: Date): string {
  return d.toISOString();
}
`

const reactOrchestrationPMemo = `"use client";

import { memo, useEffect, useState } from "react";

export default memo(function WorkspacePage() {
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

// assertWorkspacePageShape locks the P1/P-memo expected fact shape: use_state
// order, one coordinated effect transition, three ordered workspace
// branches, two imperative UI calls, and two shared panel deps ordered by
// name.
func assertWorkspacePageShape(rec semantics.ReactComponentFacts) {
	GinkgoHelper()
	Expect(rec.ClientKind).To(Equal("use_client_directive"))

	Expect(rec.UseState).To(HaveLen(3), "expected activeView, selectedId, filterText useState bindings")
	if len(rec.UseState) == 3 {
		Expect(rec.UseState[0].Binding).To(Equal("activeView"))
		Expect(rec.UseState[0].Setter).To(Equal("setActiveView"))
		Expect(rec.UseState[1].Binding).To(Equal("selectedId"))
		Expect(rec.UseState[1].Setter).To(Equal("setSelectedId"))
		Expect(rec.UseState[2].Binding).To(Equal("filterText"))
		Expect(rec.UseState[2].Setter).To(Equal("setFilterText"))
	}

	Expect(rec.CoordinatedTransitions).To(HaveLen(1), "expected exactly one coordinated effect transition")
	if len(rec.CoordinatedTransitions) == 1 {
		Expect(rec.CoordinatedTransitions[0].Kind).To(Equal("effect"))
		Expect(rec.CoordinatedTransitions[0].UpdatedBindings).To(Equal([]string{"activeView", "filterText"}))
	}

	Expect(rec.WorkspaceBranches).To(HaveLen(3), "expected list/detail/settings workspace branches")
	if len(rec.WorkspaceBranches) == 3 {
		Expect(rec.WorkspaceBranches[0].Label).To(Equal("list"))
		Expect(rec.WorkspaceBranches[1].Label).To(Equal("detail"))
		Expect(rec.WorkspaceBranches[2].Label).To(Equal("settings"))
	}

	Expect(rec.ImperativeUI).To(HaveLen(2), "expected getElementById + focus imperative UI calls")
	apis := map[string]bool{}
	for _, c := range rec.ImperativeUI {
		apis[c.API] = true
	}
	Expect(apis).To(HaveKey("getElementById"))
	Expect(apis).To(HaveKey("focus"))

	Expect(rec.SharedPanelDeps).To(HaveLen(2), "expected filterText and selectedId shared across panels")
	if len(rec.SharedPanelDeps) == 2 {
		Expect(rec.SharedPanelDeps[0].Name).To(Equal("filterText"))
		Expect(rec.SharedPanelDeps[0].Panels).To(Equal([]string{"ListPanel", "SettingsPanel"}))
		Expect(rec.SharedPanelDeps[1].Name).To(Equal("selectedId"))
		Expect(rec.SharedPanelDeps[1].Panels).To(Equal([]string{"DetailPanel", "ListPanel"}))
	}
}

var _ = Describe("React component orchestration density facts (epic #139 Story 1/2)", func() {
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

	When("a destructuring useState binding has an elided (hole) first element, as in const [, setC] = useState(2)", func() {
		It("shall keep the setter in the setter slot rather than shifting it into the binding slot", func() {
			const src = `"use client";

import { useState } from "react";

export function Counter() {
  const [, setC] = useState(2);
  const [x, setX] = useState(1);
  return <div>{x}</div>;
}
`
			result := analyzeTSX(analyzer, "Counter.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Counter")
			Expect(ok).To(BeTrue(), "expected a react_components record named Counter, got %+v", result.ReactComponents)
			Expect(rec.UseState).To(HaveLen(2))
			if len(rec.UseState) == 2 {
				Expect(rec.UseState[0].Binding).To(Equal(""))
				Expect(rec.UseState[0].Setter).To(Equal("setC"))
				Expect(rec.UseState[1].Binding).To(Equal("x"))
				Expect(rec.UseState[1].Setter).To(Equal("setX"))
			}
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
