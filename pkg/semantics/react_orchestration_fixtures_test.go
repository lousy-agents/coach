package semantics_test

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
