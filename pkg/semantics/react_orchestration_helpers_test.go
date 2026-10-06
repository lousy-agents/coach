package semantics_test

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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

// assertNonEmptyLocation locks Story 1's requirement that every recorded
// fact carry a real Tree-sitter span (start_byte < end_byte).
func assertNonEmptyLocation(loc semantics.Location, label string) {
	GinkgoHelper()
	Expect(loc.EndByte).To(BeNumerically(">", loc.StartByte), "%s location must be a non-empty span, got %+v", label, loc)
}
