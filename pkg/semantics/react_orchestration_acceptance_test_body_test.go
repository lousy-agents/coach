package semantics_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_reactOrchestrationAcceptanceTest_shallKeepTheSetterInTheSetterSlotRatherThanShift_320(analyzer *semantics.Analyzer) {
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
}

func body_reactOrchestrationAcceptanceTest_shallClassifyTheTransitionAsCallbackNotEffect_403(analyzer *semantics.Analyzer) {
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
}

func body_reactOrchestrationAcceptanceTest_shallRecordTheAssignedBindingAsTheTransitionName_431(analyzer *semantics.Analyzer) {
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
}

func body_reactOrchestrationAcceptanceTest_shallNotRecordThatIdentifierAsASharedPanelDep_460(analyzer *semantics.Analyzer) {
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
}

func body_reactOrchestrationAcceptanceTest_shallIncludeTheResidualPanelAsAThirdWorkspaceBra_526(analyzer *semantics.Analyzer) {
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
}

func body_reactOrchestrationAcceptanceTest_shallRecordAllThreeWorkspaceBranchesIncludingThe_571(analyzer *semantics.Analyzer) {
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
}

func body_reactOrchestrationAcceptanceTest_shallRecordFourWorkspaceBranchesAndKeepResidualN_614(analyzer *semantics.Analyzer) {
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
}

func body_reactOrchestrationAcceptanceTest_shallLabelThatBranchWithTheBranchSentinel_664(analyzer *semantics.Analyzer) {
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
}

func body_reactOrchestrationAcceptanceTest_shallStillEmitExactlyTheThreeEqualityWorkspaceBr_706(analyzer *semantics.Analyzer) {
	result := analyzeTSX(analyzer, "WorkspacePage.tsx", reactOrchestrationP1)

	rec, ok := reactComponentByName(result.ReactComponents, "WorkspacePage")
	Expect(ok).To(BeTrue())
	Expect(rec.WorkspaceBranches).To(HaveLen(3))
	if len(rec.WorkspaceBranches) == 3 {
		Expect(rec.WorkspaceBranches[0].Label).To(Equal("list"))
		Expect(rec.WorkspaceBranches[1].Label).To(Equal("detail"))
		Expect(rec.WorkspaceBranches[2].Label).To(Equal("settings"))
	}
}

func body_reactOrchestrationAcceptanceTest_shallAttachARecordWithClientKindHooksAndJsx_721(analyzer *semantics.Analyzer) {
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
}
