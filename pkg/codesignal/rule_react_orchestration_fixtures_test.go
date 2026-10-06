package codesignal_test

import (
	"context"

	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

// Supporting-predicate isolation fixtures: required criteria always hold
// (3 domains, >=1 coordinated transition, 3 discriminant branches). Exactly
// one supporting path holds so OR semantics and binding-count thresholds are
// pinned independently of the all-true P1 happy path.

const reactOrchestrationSupportEffectPair = `"use client";

import { useEffect, useState } from "react";

export function EffectPairOnlyPage() {
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

const reactOrchestrationSupportHandlerTriple = `"use client";

import { useState } from "react";

export function HandlerTripleOnlyPage() {
  const [activeView, setActiveView] = useState("a");
  const [selectedId, setSelectedId] = useState("x");
  const [filterText, setFilterText] = useState("");
  const onReset = () => {
    setActiveView("a");
    setSelectedId("x");
    setFilterText("");
  };
  return (
    <div>
      {activeView === "a" ? (
        <A />
      ) : activeView === "b" ? (
        <B />
      ) : activeView === "c" ? (
        <C />
      ) : null}
      <button type="button" onClick={onReset}>
        Reset
      </button>
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
`

const reactOrchestrationSupportImperativeOnly = `"use client";

import { useState } from "react";

export function ImperativeOnlyPage() {
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
        <A />
      ) : activeView === "b" ? (
        <B />
      ) : activeView === "c" ? (
        <C />
      ) : null}
      <button
        type="button"
        onClick={() => {
          document.getElementById("workspace-header")?.focus();
          onReset();
        }}
      >
        Reset
      </button>
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
`

const reactOrchestrationSupportSharedOnly = `"use client";

import { useState } from "react";

export function SharedOnlyPage() {
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
        <B selectedId={selectedId} />
      ) : activeView === "c" ? (
        <C filterText={filterText} />
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
function B(_props: { selectedId: string }) {
  return <section />;
}
function C(_props: { filterText: string }) {
  return <section />;
}
`
