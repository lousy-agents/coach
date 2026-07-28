package codesignal_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

const reactOrchestrationRuleID = "structure.react_component_orchestration_density"

const (
	reactOrchestrationEvidenceP1     = "domains=3;states=3;transitions=1;branches=3;imperative=2;shared=2;client=use_client_directive"
	reactOrchestrationEvidenceL1Head = "domains=4;states=4;transitions=1;branches=3;imperative=2;shared=2;client=use_client_directive"
	reactOrchestrationWhyItMatters   = "This component coordinates several independently changing UI concerns and feature panels. This is a refactoring opportunity, not a functional defect: changes to one workflow can unintentionally affect another, and the component is harder to test or extract safely."
	reactOrchestrationRecommendation = "Preserve the public component boundary. First extract one cohesive workspace or panel, then move only its local interaction state and derived data into that module or a dedicated hook. Keep cross-workspace state in the parent. Do not split by line count; derive explicit state-transition actions before moving state."
)

// analyzeTSXForCodesignal analyzes real TSX fixture bytes end-to-end through
// pkg/semantics, mirroring the flow codesignal rules actually run on
// (locking Story 2/5's real fixture->Result->Signal path, not a hand-built
// semantics.Result literal).
func analyzeTSXForCodesignal(path, source string) *semantics.Result {
	GinkgoHelper()
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	Expect(err).NotTo(HaveOccurred())
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

const reactOrchestrationRuleN1 = `"use client";

import { useState } from "react";

export function HugeForm() {
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [notes, setNotes] = useState("");
  return (
    <form>
      <input value={firstName} onChange={(e) => setFirstName(e.target.value)} />
      <input value={lastName} onChange={(e) => setLastName(e.target.value)} />
      <input value={email} onChange={(e) => setEmail(e.target.value)} />
      <input value={phone} onChange={(e) => setPhone(e.target.value)} />
      <textarea value={notes} onChange={(e) => setNotes(e.target.value)} />
    </form>
  );
}
`

const reactOrchestrationRuleN2 = `"use client";

import { useState } from "react";

export function DataTable() {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState("name");
  const [pageIndex, setPageIndex] = useState(0);
  return (
    <div>
      <button type="button" onClick={() => setSortKey("name")}>
        Sort
      </button>
      <button type="button" onClick={() => setPageIndex(pageIndex + 1)}>
        Next
      </button>
      <button type="button" onClick={() => setSelectedId("1")}>
        Select
      </button>
      <table>
        <tbody>
          <tr>
            <td>{selectedId}</td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}
`

const reactOrchestrationRuleN3 = `"use client";

import { useState } from "react";

export function SimpleTabs() {
  const [activeView, setActiveView] = useState("a");
  return (
    <div>
      {activeView === "a" ? (
        <PanelA />
      ) : activeView === "b" ? (
        <PanelB />
      ) : activeView === "c" ? (
        <PanelC />
      ) : null}
      <button type="button" onClick={() => setActiveView("b")}>
        B
      </button>
    </div>
  );
}

function PanelA() {
  return <section />;
}
function PanelB() {
  return <section />;
}
function PanelC() {
  return <section />;
}
`

const reactOrchestrationRuleN4 = `"use client";

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

const reactOrchestrationRuleN5 = `export function ServerPage(props: { children: string }) {
  return <div>{props.children}</div>;
}
`

const reactOrchestrationRuleN6Go = `package p

func WorkspacePage() { println("useState") }
`

const reactOrchestrationRuleN7 = `"use client";

import { useEffect, useState } from "react";

export function TwoDomainPage() {
  const [activeView, setActiveView] = useState("a");
  const [selectedId, setSelectedId] = useState("x");
  useEffect(() => {
    setActiveView("a");
    setSelectedId("x");
  }, []);
  return activeView === "a" ? (
    <A />
  ) : activeView === "b" ? (
    <B />
  ) : activeView === "c" ? (
    <C />
  ) : null;
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

const reactOrchestrationRuleN8 = `"use client";

import { useState } from "react";

export function HandlerOnlyPage() {
  const [activeView, setActiveView] = useState("a");
  const [selectedId, setSelectedId] = useState("x");
  const [filterText, setFilterText] = useState("");
  const onReset = () => {
    setActiveView("a");
    setSelectedId("x");
  };
  return (
    <div>
      {activeView === "a" ? (
        <A selectedId={selectedId} />
      ) : activeView === "b" ? (
        <B filterText={filterText} />
      ) : activeView === "c" ? (
        <C />
      ) : null}
      <button type="button" onClick={onReset}>
        Reset
      </button>
    </div>
  );
}

function A(_props: { selectedId: string }) {
  return <section />;
}
function B(_props: { filterText: string }) {
  return <section />;
}
function C() {
  return <section />;
}
`

const reactOrchestrationRuleN9 = `"use client";

export function formatDate(d: Date): string {
  return d.toISOString();
}
`

const reactOrchestrationRuleP1 = `"use client";

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

const reactOrchestrationRuleP1WithDraft = `"use client";

import { useEffect, useState } from "react";

export function WorkspacePage() {
  const [activeView, setActiveView] = useState("list");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [filterText, setFilterText] = useState("");
  const [workspaceDraft, setWorkspaceDraft] = useState("");
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

const reactOrchestrationRulePMemo = `"use client";

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

const reactOrchestrationRuleDomainOrder = `"use client";

import { useEffect, useState } from "react";

export function DomainOrderPage() {
  const [activeView, setActiveView] = useState("list");
  const [selectedView, setSelectedView] = useState("list");
  const [filterText, setFilterText] = useState("");
  const [hoverRow, setHoverRow] = useState<string | null>(null);
  const header = document.getElementById("workspace-header");

  useEffect(() => {
    setFilterText("");
    setActiveView("list");
  }, [selectedView]);

  return (
    <div>
      {activeView === "list" ? (
        <ListPanel
          selectedView={selectedView}
          filterText={filterText}
          hoverRow={hoverRow}
          onSelect={setSelectedView}
          onHover={setHoverRow}
        />
      ) : activeView === "detail" ? (
        <DetailPanel selectedView={selectedView} onBack={() => setActiveView("list")} />
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
  selectedView: string;
  filterText: string;
  hoverRow: string | null;
  onSelect: (id: string) => void;
  onHover: (id: string | null) => void;
}) {
  return <section />;
}
function DetailPanel(_props: { selectedView: string; onBack: () => void }) {
  return <section />;
}
function SettingsPanel(_props: { filterText: string; onFilter: (v: string) => void }) {
  return <section />;
}
`

const reactOrchestrationRuleElidedHole = `"use client";

import { useEffect, useState } from "react";

export function TwoDomainWithHolePage() {
  const [activeView, setActiveView] = useState("a");
  const [selectedId, setSelectedId] = useState("x");
  const [, setSomething] = useState(0);
  useEffect(() => {
    setActiveView("a");
    setSelectedId("x");
  }, []);
  return activeView === "a" ? (
    <A />
  ) : activeView === "b" ? (
    <B />
  ) : activeView === "c" ? (
    <C />
  ) : null;
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

var _ = Describe("Story 5: structure.react_component_orchestration_density codesignal rule", func() {
	When("P1: WorkspacePage.tsx is analyzed end to end through semantics then codesignal", func() {
		It("shall emit exactly one signal with the locked field shape", func() {
			head := analyzeTSXForCodesignal("WorkspacePage.tsx", reactOrchestrationRuleP1)
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "WorkspacePage.tsx", Status: "modified", Head: head,
			}}})

			signals := signalsByRule(report, reactOrchestrationRuleID)
			Expect(signals).To(HaveLen(1), "expected exactly one react_component_orchestration_density signal for P1")

			sig := signals[0]
			Expect(sig.RuleID).To(Equal(reactOrchestrationRuleID))
			Expect(sig.RuleVersion).To(Equal("1"))
			Expect(sig.Kind).To(Equal("react_component_orchestration_density"))
			Expect(sig.Category).To(Equal(codesignal.Category("structure")))
			Expect(sig.Severity).To(Equal(codesignal.Severity("medium")))
			Expect(sig.Confidence).To(Equal(codesignal.Confidence("high")))
			Expect(sig.Subject).To(Equal("WorkspacePage"))
			Expect(sig.Evidence).To(Equal(reactOrchestrationEvidenceP1))
			Expect(sig.WhyItMatters).To(Equal(reactOrchestrationWhyItMatters))
			Expect(sig.Recommendation).To(Equal(reactOrchestrationRecommendation))
			Expect(sig.Provenance).To(Equal(codesignal.Provenance{Producer: "codesignal"}))
		})
	})

	When("P-memo: the same component is wrapped in memo(...)", func() {
		It("shall emit one signal with the same subject and evidence as P1", func() {
			head := analyzeTSXForCodesignal("WorkspacePageMemo.tsx", reactOrchestrationRulePMemo)
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "WorkspacePageMemo.tsx", Status: "modified", Head: head,
			}}})

			signals := signalsByRule(report, reactOrchestrationRuleID)
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Subject).To(Equal("WorkspacePage"))
			Expect(signals[0].Evidence).To(Equal(reactOrchestrationEvidenceP1))
		})
	})

	When("Domain order: activeView/selectedView/filterText/hoverRow bindings are analyzed", func() {
		It("shall collapse selectedView into the navigation domain (not a distinct selection domain), yielding 3 unique domains", func() {
			head := analyzeTSXForCodesignal("DomainOrderPage.tsx", reactOrchestrationRuleDomainOrder)
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "DomainOrderPage.tsx", Status: "modified", Head: head,
			}}})

			signals := signalsByRule(report, reactOrchestrationRuleID)
			Expect(signals).To(HaveLen(1))
			// Under correct domain-table ordering, "navigation" is matched
			// before "selection", so selectedView collapses into the same
			// domain as activeView: {navigation, filtering, hover_focus} = 3
			// unique domains. A swapped table would classify selectedView as
			// "selection" instead, producing 4 unique domains and failing
			// this assertion.
			Expect(signals[0].Evidence).To(ContainSubstring("domains=3"))
		})
	})

	DescribeTable("silent fixtures shall emit no react_component_orchestration_density signal",
		func(path, source string, language semantics.Language) {
			var head *semantics.Result
			if language == semantics.LanguageGo {
				analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
				Expect(err).NotTo(HaveOccurred())
				result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
					Path: path, Language: semantics.LanguageGo, Content: []byte(source),
				})
				Expect(err).NotTo(HaveOccurred())
				head = result
			} else {
				head = analyzeTSXForCodesignal(path, source)
			}

			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: path, Status: "modified", Head: head,
			}}})
			Expect(signalsByRule(report, reactOrchestrationRuleID)).To(BeEmpty())
		},
		Entry("N1: five other-domain bindings, uniqueDomains=1", "HugeForm.tsx", reactOrchestrationRuleN1, semantics.LanguageTSX),
		Entry("N2: no multi-setter transition, branches<3", "DataTable.tsx", reactOrchestrationRuleN2, semantics.LanguageTSX),
		Entry("N3: uniqueDomains<3", "SimpleTabs.tsx", reactOrchestrationRuleN3, semantics.LanguageTSX),
		Entry("N4: orchestration lives in a non-component helper", "N4Page.tsx", reactOrchestrationRuleN4, semantics.LanguageTSX),
		Entry("N5: server component, no hooks", "ServerPage.tsx", reactOrchestrationRuleN5, semantics.LanguageTSX),
		Entry("N6: Go source, no React constructs", "page.go", reactOrchestrationRuleN6Go, semantics.LanguageGo),
		Entry("N7: only two unique domains", "TwoDomainPage.tsx", reactOrchestrationRuleN7, semantics.LanguageTSX),
		Entry("N8: supporting predicates A/B/C all fail", "HandlerOnlyPage.tsx", reactOrchestrationRuleN8, semantics.LanguageTSX),
		Entry("N9: use client directive but no JSX", "formatDate.tsx", reactOrchestrationRuleN9, semantics.LanguageTSX),
	)

	When("N8-class required predicates pass but only a module-level non-state identifier is shared across panels", func() {
		It("shall emit no signal (Supporting C must not fire on non-state shared identifiers)", func() {
			const src = `"use client";

import { useState } from "react";

const theme = "dark";

export function HandlerOnlyPage() {
  const [activeView, setActiveView] = useState("a");
  const [selectedId, setSelectedId] = useState("x");
  const [filterText, setFilterText] = useState("");
  const onReset = () => {
    setActiveView("a");
    setSelectedId("x");
  };
  return (
    <div>
      {activeView === "a" ? (
        <A selectedId={selectedId} theme={theme} />
      ) : activeView === "b" ? (
        <B filterText={filterText} theme={theme} />
      ) : activeView === "c" ? (
        <C />
      ) : null}
      <button type="button" onClick={onReset}>
        Reset
      </button>
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
			head := analyzeTSXForCodesignal("HandlerOnlyPage.tsx", src)
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "HandlerOnlyPage.tsx", Status: "modified", Head: head,
			}}})
			Expect(signalsByRule(report, reactOrchestrationRuleID)).To(BeEmpty(),
				"shared module-level theme must not satisfy Supporting C; got signals %+v",
				signalsByRule(report, reactOrchestrationRuleID))
		})
	})

	When("an elided-hole useState binding (const [, setSomething] = useState(...)) is present alongside exactly two real, distinct state domains", func() {
		It("shall emit no signal (the empty binding name must not be classified into a phantom 'other' domain)", func() {
			head := analyzeTSXForCodesignal("TwoDomainWithHolePage.tsx", reactOrchestrationRuleElidedHole)
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "TwoDomainWithHolePage.tsx", Status: "modified", Head: head,
			}}})
			signals := signalsByRule(report, reactOrchestrationRuleID)
			Expect(signals).To(BeEmpty(),
				"activeView/selectedId classify into only 2 real domains (navigation, selection); "+
					"the elided-hole useState binding must not contribute a phantom 'other' domain that "+
					"would push uniqueDomains to 3 and satisfy the required domain-count criterion; got signals %+v",
				signals)
		})
	})

	When("L1: WorkspacePage.tsx gains a fourth useState binding between base and head", func() {
		It("shall mark the base evidence resolved and the head evidence introduced", func() {
			base := analyzeTSXForCodesignal("WorkspacePage.tsx", reactOrchestrationRuleP1)
			head := analyzeTSXForCodesignal("WorkspacePage.tsx", reactOrchestrationRuleP1WithDraft)

			report := build(codesignal.Options{IncludeResolved: true}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "WorkspacePage.tsx", Status: "modified", Base: base, Head: head,
			}}})

			signals := signalsByRule(report, reactOrchestrationRuleID)
			Expect(signals).To(HaveLen(2), "expected a resolved base signal and an introduced head signal (distinct evidence keys)")

			byLife := map[codesignal.Lifecycle]codesignal.Signal{}
			for _, s := range signals {
				byLife[s.Lifecycle] = s
			}
			Expect(byLife[codesignal.Lifecycle("resolved")].Evidence).To(Equal(reactOrchestrationEvidenceP1))
			Expect(byLife[codesignal.Lifecycle("introduced")].Evidence).To(Equal(reactOrchestrationEvidenceL1Head))
		})
	})
})
