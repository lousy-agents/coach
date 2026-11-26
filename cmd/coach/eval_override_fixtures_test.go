package main

import "strings"

const nestedGoBelowEveryDefaultThreshold = `package fixture

func Nested(xs []int) int {
	total := 0
	for _, x := range xs {
		if x > 0 {
			total += x
		}
	}
	return total
}

type Thing struct{}

func NewThing() *Thing { return &Thing{} }
`

const nestedGoAtTheDefaultNestingThreshold = `package fixture

func Nested(xs []int, ys []int) int {
	total := 0
	if len(xs) > 0 {
		for _, x := range xs {
			if x > 0 {
				total += x
			}
		}
	}
	return total
}
`

const importTwoLevelsUp = `import { x } from "../../x";
export const y = x;
`

const importThreeLevelsUp = `import { x } from "../../../x";
export const y = x;
`

const workspaceWithThreeStateDomains = `"use client";

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
function SettingsPanel(_props: { filterText: string; onFilter: (value: string) => void }) {
  return <section />;
}
`

var workspaceWithTwoStateDomains = strings.NewReplacer("filterText", "activeTab", "setFilterText", "setActiveTab").Replace(workspaceWithThreeStateDomains)
