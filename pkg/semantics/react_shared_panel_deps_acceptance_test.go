package semantics_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

var _ = Describe("React component orchestration density facts (epic #139 Story 1/2): shared panel deps", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
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
})
