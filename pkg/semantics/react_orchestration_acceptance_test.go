package semantics_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

// assertNonEmptyLocation locks Story 1's requirement that every recorded
// fact carry a real Tree-sitter span (start_byte < end_byte).
func assertNonEmptyLocation(loc semantics.Location, label string) {
	GinkgoHelper()
	Expect(loc.EndByte).To(BeNumerically(">", loc.StartByte), "%s location must be a non-empty span, got %+v", label, loc)
}

// assertWorkspacePageShape locks the P1/P-memo expected fact shape: use_state
// order, one coordinated effect transition, three ordered workspace
// branches, two imperative UI calls, and two shared panel deps ordered by
// name.
func assertWorkspacePageShape(rec semantics.ReactComponentFacts) {
	GinkgoHelper()
	Expect(rec.ClientKind).To(Equal("use_client_directive"))
	assertNonEmptyLocation(rec.Location, "component")

	Expect(rec.UseState).To(HaveLen(3), "expected activeView, selectedId, filterText useState bindings")
	if len(rec.UseState) == 3 {
		Expect(rec.UseState[0].Binding).To(Equal("activeView"))
		Expect(rec.UseState[0].Setter).To(Equal("setActiveView"))
		assertNonEmptyLocation(rec.UseState[0].Location, "use_state[0]")
		Expect(rec.UseState[1].Binding).To(Equal("selectedId"))
		Expect(rec.UseState[1].Setter).To(Equal("setSelectedId"))
		assertNonEmptyLocation(rec.UseState[1].Location, "use_state[1]")
		Expect(rec.UseState[2].Binding).To(Equal("filterText"))
		Expect(rec.UseState[2].Setter).To(Equal("setFilterText"))
		assertNonEmptyLocation(rec.UseState[2].Location, "use_state[2]")
	}

	Expect(rec.CoordinatedTransitions).To(HaveLen(1), "expected exactly one coordinated effect transition")
	if len(rec.CoordinatedTransitions) == 1 {
		Expect(rec.CoordinatedTransitions[0].Kind).To(Equal("effect"))
		Expect(rec.CoordinatedTransitions[0].Name).To(Equal("<anonymous>"), "anonymous effect callbacks must use the <anonymous> name sentinel, not empty string")
		Expect(rec.CoordinatedTransitions[0].UpdatedBindings).To(Equal([]string{"activeView", "filterText"}))
		assertNonEmptyLocation(rec.CoordinatedTransitions[0].Location, "coordinated_transitions[0]")
	}

	Expect(rec.WorkspaceBranches).To(HaveLen(3), "expected list/detail/settings workspace branches")
	if len(rec.WorkspaceBranches) == 3 {
		Expect(rec.WorkspaceBranches[0].Label).To(Equal("list"))
		Expect(rec.WorkspaceBranches[1].Label).To(Equal("detail"))
		Expect(rec.WorkspaceBranches[2].Label).To(Equal("settings"))
		assertNonEmptyLocation(rec.WorkspaceBranches[0].Location, "workspace_branches[0]")
		assertNonEmptyLocation(rec.WorkspaceBranches[1].Location, "workspace_branches[1]")
		assertNonEmptyLocation(rec.WorkspaceBranches[2].Location, "workspace_branches[2]")
	}

	Expect(rec.ImperativeUI).To(HaveLen(2), "expected getElementById + focus imperative UI calls")
	apis := map[string]bool{}
	for i, c := range rec.ImperativeUI {
		apis[c.API] = true
		assertNonEmptyLocation(c.Location, fmt.Sprintf("imperative_ui[%d]", i))
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

	When("a nested PascalCase inner component calls two useState setters from the outer scope", func() {
		It("shall attribute no coordinated transition to the outer component", func() {
			const src = `"use client";

import { useState } from "react";

export function Outer() {
  const [a, setA] = useState(1);
  const [b, setB] = useState(2);
  function InnerPanel() {
    setA(1);
    setB(2);
    return <section />;
  }
  return (
    <div>
      <InnerPanel />
    </div>
  );
}
`
			result := analyzeTSX(analyzer, "Outer.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Outer")
			Expect(ok).To(BeTrue(), "expected a react_components record named Outer, got %+v", result.ReactComponents)
			Expect(rec.CoordinatedTransitions).To(BeEmpty(), "InnerPanel's setter calls must not attribute a coordinated transition to Outer, got %+v", rec.CoordinatedTransitions)
		})
	})

	When("a ternary chain's branches test three structurally distinct discriminant bases (v, props.v, other.v)", func() {
		It("shall not treat the mixed-base chain as a single >=3 branch workspace-branch chain", func() {
			const src = `"use client";

export function Mixed(props: { v: string }) {
  const v = "x";
  const other = { v: "z" };
  return (
    <div>
      {v === "a" ? (
        <Aa />
      ) : props.v === "b" ? (
        <Bb />
      ) : other.v === "c" ? (
        <Cc />
      ) : null}
    </div>
  );
}
`
			result := analyzeTSX(analyzer, "Mixed.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Mixed")
			Expect(ok).To(BeTrue(), "expected a react_components record named Mixed, got %+v", result.ReactComponents)
			Expect(rec.WorkspaceBranches).To(BeEmpty(), "mixed-base discriminant chain (v vs props.v vs other.v) must contribute zero branches, got %+v", rec.WorkspaceBranches)
		})
	})

	When("a two-setter-calling arrow function is the second (not first) argument of a useEffect call", func() {
		It("shall classify the transition as callback, not effect", func() {
			const src = `"use client";

import { useState } from "react";

export function Counter() {
  const [a, setA] = useState(1);
  const [b, setB] = useState(2);
  useEffect(null, () => {
    setA(1);
    setB(2);
  });
  return <div>{a}</div>;
}
`
			result := analyzeTSX(analyzer, "Counter.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Counter")
			Expect(ok).To(BeTrue(), "expected a react_components record named Counter, got %+v", result.ReactComponents)
			Expect(rec.CoordinatedTransitions).To(HaveLen(1), "expected exactly one coordinated transition")
			if len(rec.CoordinatedTransitions) == 1 {
				Expect(rec.CoordinatedTransitions[0].Kind).To(Equal("callback"), "an arrow function in the second argument position of useEffect must not be classified as an effect")
				Expect(rec.CoordinatedTransitions[0].Name).To(Equal("<anonymous>"), "anonymous callbacks must use the <anonymous> name sentinel, not empty string")
			}
		})
	})

	When("a non-handler-named local callback updates two state bindings", func() {
		It("shall record the assigned binding as the transition name with kind callback", func() {
			const src = `"use client";

import { useState } from "react";

export function Counter() {
  const [a, setA] = useState(1);
  const [b, setB] = useState(2);
  const run = () => {
    setA(1);
    setB(2);
  };
  return <button type="button" onClick={run} />;
}
`
			result := analyzeTSX(analyzer, "Counter.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Counter")
			Expect(ok).To(BeTrue(), "expected a react_components record named Counter, got %+v", result.ReactComponents)
			Expect(rec.CoordinatedTransitions).To(HaveLen(1))
			if len(rec.CoordinatedTransitions) == 1 {
				Expect(rec.CoordinatedTransitions[0].Kind).To(Equal("callback"))
				Expect(rec.CoordinatedTransitions[0].Name).To(Equal("run"), "assigned non-on*/handle* callbacks must keep their binding name")
				Expect(rec.CoordinatedTransitions[0].UpdatedBindings).To(Equal([]string{"a", "b"}))
			}
		})
	})

	When("a module-level non-state identifier is passed to two panels but no state binding is shared", func() {
		It("shall not record that identifier as a shared_panel_dep", func() {
			const src = `"use client";

import { useState } from "react";

const theme = "dark";

export function Page() {
  const [activeView, setActiveView] = useState("a");
  const [selectedId, setSelectedId] = useState("x");
  const [filterText, setFilterText] = useState("");
  return (
    <div>
      {activeView === "a" ? (
        <A selectedId={selectedId} theme={theme} />
      ) : activeView === "b" ? (
        <B filterText={filterText} theme={theme} />
      ) : activeView === "c" ? (
        <C />
      ) : null}
    </div>
  );
}

function A(_props: { selectedId: string; theme: string }) {
  return <section />;
}
function B(_props: { filterText: string; theme: string }) {
  return <section />;
}
function C() {
  return <section />;
}
`
			result := analyzeTSX(analyzer, "Page.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Page")
			Expect(ok).To(BeTrue(), "expected a react_components record named Page, got %+v", result.ReactComponents)
			for _, dep := range rec.SharedPanelDeps {
				Expect(dep.Name).NotTo(Equal("theme"), "module-level non-state identifiers must not become shared_panel_deps, got %+v", rec.SharedPanelDeps)
			}
			Expect(rec.SharedPanelDeps).To(BeEmpty(), "no state binding or known callback is shared across >=2 panels, got %+v", rec.SharedPanelDeps)
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

	When("a discriminant ternary chain ends in a residual capitalized JSX panel (no equality arm)", func() {
		It("shall include the residual panel as a third workspace branch labeled with the component name", func() {
			const src = `"use client";

import { useState } from "react";

export function Page() {
  const [activeView, setActiveView] = useState("a");
  return (
    <div>
      {activeView === "a" ? (
        <A />
      ) : activeView === "b" ? (
        <B />
      ) : (
        <DefaultPanel />
      )}
    </div>
  );
}

function A() {
  return <section />;
}
function B() {
  return <section />;
}
function DefaultPanel() {
  return <section />;
}
`
			result := analyzeTSX(analyzer, "ResidualTernary.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Page")
			Expect(ok).To(BeTrue(), "expected a react_components record named Page, got %+v", result.ReactComponents)
			Expect(rec.WorkspaceBranches).To(HaveLen(3), "two equality arms plus residual DefaultPanel must yield 3 branches, got %+v", rec.WorkspaceBranches)
			if len(rec.WorkspaceBranches) == 3 {
				Expect(rec.WorkspaceBranches[0].Label).To(Equal("a"))
				Expect(rec.WorkspaceBranches[1].Label).To(Equal("b"))
				Expect(rec.WorkspaceBranches[2].Label).To(Equal("DefaultPanel"),
					"residual capitalized primary JSX child must label the branch, got %+v", rec.WorkspaceBranches[2])
			}
		})
	})

	When("an if/else-if/else chain has three JSX-bearing arms including a final else", func() {
		It("shall record all three workspace branches including the final else panel", func() {
			const src = `"use client";

import { useState } from "react";

export function Page() {
  const [activeView, setActiveView] = useState("a");
  let panel;
  if (activeView === "a") {
    panel = <A />;
  } else if (activeView === "b") {
    panel = <B />;
  } else {
    panel = <C />;
  }
  return <div>{panel}</div>;
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
			result := analyzeTSX(analyzer, "IfElseFinal.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Page")
			Expect(ok).To(BeTrue(), "expected a react_components record named Page, got %+v", result.ReactComponents)
			Expect(rec.WorkspaceBranches).To(HaveLen(3), "if/else-if/else with three JSX arms must yield 3 branches, got %+v", rec.WorkspaceBranches)
			if len(rec.WorkspaceBranches) == 3 {
				Expect(rec.WorkspaceBranches[0].Label).To(Equal("a"))
				Expect(rec.WorkspaceBranches[1].Label).To(Equal("b"))
				Expect(rec.WorkspaceBranches[2].Label).To(Equal("C"),
					"final else capitalized primary JSX child must label the branch, got %+v", rec.WorkspaceBranches[2])
			}
		})
	})

	When("a ternary chain has three equality arms plus a residual capitalized panel", func() {
		It("shall record four workspace branches and keep residual null/non-JSX from counting", func() {
			const src = `"use client";

import { useState } from "react";

export function Page() {
  const [activeView, setActiveView] = useState("a");
  return (
    <div>
      {activeView === "a" ? (
        <A />
      ) : activeView === "b" ? (
        <B />
      ) : activeView === "c" ? (
        <C />
      ) : (
        <DefaultPanel />
      )}
    </div>
  );
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
function DefaultPanel() {
  return <section />;
}
`
			result := analyzeTSX(analyzer, "ThreePlusResidual.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Page")
			Expect(ok).To(BeTrue(), "expected a react_components record named Page, got %+v", result.ReactComponents)
			Expect(rec.WorkspaceBranches).To(HaveLen(4), "three equality arms plus residual must yield 4 branches, got %+v", rec.WorkspaceBranches)
			if len(rec.WorkspaceBranches) == 4 {
				Expect(rec.WorkspaceBranches[0].Label).To(Equal("a"))
				Expect(rec.WorkspaceBranches[1].Label).To(Equal("b"))
				Expect(rec.WorkspaceBranches[2].Label).To(Equal("c"))
				Expect(rec.WorkspaceBranches[3].Label).To(Equal("DefaultPanel"))
			}
		})
	})

	When("a residual ternary arm's primary JSX child is a non-PascalCase element", func() {
		It("shall label that branch with the <branch> sentinel", func() {
			const src = `"use client";

import { useState } from "react";

export function Page() {
  const [activeView, setActiveView] = useState("a");
  return (
    <div>
      {activeView === "a" ? (
        <A />
      ) : activeView === "b" ? (
        <B />
      ) : (
        <div role="region">fallback</div>
      )}
    </div>
  );
}

function A() {
  return <section />;
}
function B() {
  return <section />;
}
`
			result := analyzeTSX(analyzer, "ResidualDiv.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Page")
			Expect(ok).To(BeTrue(), "expected a react_components record named Page, got %+v", result.ReactComponents)
			Expect(rec.WorkspaceBranches).To(HaveLen(3), "two equality arms plus residual div must yield 3 branches, got %+v", rec.WorkspaceBranches)
			if len(rec.WorkspaceBranches) == 3 {
				Expect(rec.WorkspaceBranches[0].Label).To(Equal("a"))
				Expect(rec.WorkspaceBranches[1].Label).To(Equal("b"))
				Expect(rec.WorkspaceBranches[2].Label).To(Equal("<branch>"),
					"residual non-PascalCase primary JSX must use the <branch> sentinel, got %+v", rec.WorkspaceBranches[2])
			}
		})
	})

	When("P1's residual arm is null (non-JSX)", func() {
		It("shall still emit exactly the three equality workspace branches and not invent a residual", func() {
			result := analyzeTSX(analyzer, "WorkspacePage.tsx", reactOrchestrationP1)

			rec, ok := reactComponentByName(result.ReactComponents, "WorkspacePage")
			Expect(ok).To(BeTrue())
			Expect(rec.WorkspaceBranches).To(HaveLen(3))
			if len(rec.WorkspaceBranches) == 3 {
				Expect(rec.WorkspaceBranches[0].Label).To(Equal("list"))
				Expect(rec.WorkspaceBranches[1].Label).To(Equal("detail"))
				Expect(rec.WorkspaceBranches[2].Label).To(Equal("settings"))
			}
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

	When("a component body contains role=\"tabpanel\" elements", func() {
		It("shall emit one workspace branch per tabpanel with aria-label/id/tabpanel label precedence", func() {
			const src = `"use client";

export function TabHost() {
  return (
    <div>
      <div role="tabpanel" aria-label="one">
        a
      </div>
      <div role="tabpanel" id="two">
        b
      </div>
      <div role="tabpanel">c</div>
    </div>
  );
}
`
			result := analyzeTSX(analyzer, "TabHost.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "TabHost")
			Expect(ok).To(BeTrue(), "expected a react_components record named TabHost, got %+v", result.ReactComponents)
			Expect(rec.WorkspaceBranches).To(HaveLen(3), "each role=tabpanel must yield one branch, got %+v", rec.WorkspaceBranches)
			if len(rec.WorkspaceBranches) == 3 {
				Expect(rec.WorkspaceBranches[0].Label).To(Equal("one"))
				Expect(rec.WorkspaceBranches[1].Label).To(Equal("two"))
				Expect(rec.WorkspaceBranches[2].Label).To(Equal("tabpanel"))
				assertNonEmptyLocation(rec.WorkspaceBranches[0].Location, "tabpanel[0]")
				assertNonEmptyLocation(rec.WorkspaceBranches[1].Location, "tabpanel[1]")
				assertNonEmptyLocation(rec.WorkspaceBranches[2].Location, "tabpanel[2]")
			}
		})
	})

	When("an inline JSX onClick arrow updates two state bindings", func() {
		It("shall record a coordinated transition with kind handler and name onClick", func() {
			const src = `"use client";

import { useState } from "react";

export function Counter() {
  const [a, setA] = useState(1);
  const [b, setB] = useState(2);
  return (
    <button
      type="button"
      onClick={() => {
        setA(1);
        setB(2);
      }}
    />
  );
}
`
			result := analyzeTSX(analyzer, "Counter.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Counter")
			Expect(ok).To(BeTrue(), "expected a react_components record named Counter, got %+v", result.ReactComponents)
			Expect(rec.CoordinatedTransitions).To(HaveLen(1))
			if len(rec.CoordinatedTransitions) == 1 {
				Expect(rec.CoordinatedTransitions[0].Kind).To(Equal("handler"))
				Expect(rec.CoordinatedTransitions[0].Name).To(Equal("onClick"))
				Expect(rec.CoordinatedTransitions[0].UpdatedBindings).To(Equal([]string{"a", "b"}))
				assertNonEmptyLocation(rec.CoordinatedTransitions[0].Location, "onClick handler")
			}
		})
	})

	When("a local handle* binding updates two state bindings", func() {
		It("shall record a coordinated transition with kind handler and the binding name", func() {
			const src = `"use client";

import { useState } from "react";

export function Counter() {
  const [a, setA] = useState(1);
  const [b, setB] = useState(2);
  const handleReset = () => {
    setA(1);
    setB(2);
  };
  return <button type="button" onClick={handleReset} />;
}
`
			result := analyzeTSX(analyzer, "Counter.tsx", src)

			rec, ok := reactComponentByName(result.ReactComponents, "Counter")
			Expect(ok).To(BeTrue(), "expected a react_components record named Counter, got %+v", result.ReactComponents)
			Expect(rec.CoordinatedTransitions).To(HaveLen(1))
			if len(rec.CoordinatedTransitions) == 1 {
				Expect(rec.CoordinatedTransitions[0].Kind).To(Equal("handler"))
				Expect(rec.CoordinatedTransitions[0].Name).To(Equal("handleReset"))
				Expect(rec.CoordinatedTransitions[0].UpdatedBindings).To(Equal([]string{"a", "b"}))
			}
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
})
