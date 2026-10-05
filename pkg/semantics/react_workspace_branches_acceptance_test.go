package semantics_test

import (
	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("React component orchestration density facts (epic #139 Story 1/2): workspace branches from ternary discriminant chains", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
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
})

var _ = Describe("React component orchestration density facts (epic #139 Story 1/2): workspace branches from if/else-if chains", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
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
})

var _ = Describe("React component orchestration density facts (epic #139 Story 1/2): workspace branches from tabpanels", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
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
})
