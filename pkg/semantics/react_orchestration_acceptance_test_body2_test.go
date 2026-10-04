package semantics_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_reactOrchestrationAcceptanceTest_shallEmitOneWorkspaceBranchPerTabpanelWithAriaLa_810(analyzer *semantics.Analyzer) {
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
}

func body_reactOrchestrationAcceptanceTest_shallRecordACoordinatedTransitionWithKindHandler_844(analyzer *semantics.Analyzer) {
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
}

func body_reactOrchestrationAcceptanceTest_shallRecordACoordinatedTransitionWithKindHandler_878(analyzer *semantics.Analyzer) {
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
}
