package projectmodel_test

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func baseModel() projectmodel.Model {
	return projectmodel.Model{
		SchemaVersion: projectmodel.SchemaVersion,
		Snapshot: projectmodel.Snapshot{
			Revision:      "revision-sha",
			TreeID:        "tree-sha",
			ConfigDigest:  "config-digest",
			BackendDigest: "backend-digest",
		},
		Coverage: projectmodel.Coverage{Phase: "workspace_discovery", Complete: true},
	}
}

var _ = Describe("Model JSON encoding contract", func() {
	When("a Model declares its frozen schema version", func() {
		It("marshals schema_version as the SchemaVersion constant", func() {
			Expect(projectmodel.SchemaVersion).To(Equal("1"))

			out, err := json.Marshal(baseModel())
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring(`"schema_version":"1"`))
		})
	})

	When("Coverage.Counts and Coverage.Budgets contain multiple keys", func() {
		It("marshals map keys in sorted order regardless of insertion order", func() {
			model := baseModel()
			model.Coverage.Counts = map[string]int{"zebra": 1, "apple": 2, "mango": 3}
			model.Coverage.Budgets = map[string]int{"zulu": 9, "alpha": 1, "middle": 5}

			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(out)).To(ContainSubstring(`"counts":{"apple":2,"mango":3,"zebra":1}`))
			Expect(string(out)).To(ContainSubstring(`"budgets":{"alpha":1,"middle":5,"zulu":9}`))
		})
	})

	When("Snapshot contains immutable revision identities", func() {
		It("round-trips every identity unchanged", func() {
			model := baseModel()
			model.Snapshot = projectmodel.Snapshot{
				Revision:      "revision-2",
				TreeID:        "tree-2",
				ConfigDigest:  "config-2",
				BackendDigest: "backend-2",
			}

			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())

			var decoded projectmodel.Model
			Expect(json.Unmarshal(out, &decoded)).To(Succeed())
			Expect(decoded.Snapshot).To(Equal(model.Snapshot))
		})
	})

	// F-007: optional snapshot/file context must survive round-trip and stay
	// independent of absolute checkout roots.
	When("Snapshot carries selected roots, build context, and file content hashes", func() {
		It("marshals identical bytes from different absolute checkout roots", func() {
			left := baseModel()
			left.Snapshot.BuildContextDigest = "build-ctx-1"
			left.Snapshot.SelectedRoots = []string{"services/b", "services/a", "."}
			left.Files = []projectmodel.File{{
				ID: "file:a.go", Path: "services/a/a.go", Language: "go",
				BlobHash: "blob-a", ContentHash: "content-a",
			}}

			right := baseModel()
			right.Snapshot.BuildContextDigest = "build-ctx-1"
			right.Snapshot.SelectedRoots = []string{".", "services/a", "services/b"}
			right.Files = []projectmodel.File{{
				ID: "file:a.go", Path: "services/a/a.go", Language: "go",
				BlobHash: "blob-a", ContentHash: "content-a",
			}}

			leftJSON, err := json.Marshal(left)
			Expect(err).NotTo(HaveOccurred())
			rightJSON, err := json.Marshal(right)
			Expect(err).NotTo(HaveOccurred())
			Expect(leftJSON).To(Equal(rightJSON))
			Expect(string(leftJSON)).To(ContainSubstring(`"selected_roots":[".","services/a","services/b"]`))
			Expect(string(leftJSON)).To(ContainSubstring(`"build_context_digest":"build-ctx-1"`))
			Expect(string(leftJSON)).To(ContainSubstring(`"blob_hash":"blob-a"`))
			Expect(string(leftJSON)).To(ContainSubstring(`"content_hash":"content-a"`))
			Expect(string(leftJSON)).NotTo(ContainSubstring("/var/"))
			Expect(string(leftJSON)).NotTo(ContainSubstring("/tmp/"))

			var decoded projectmodel.Model
			Expect(json.Unmarshal(leftJSON, &decoded)).To(Succeed())
			Expect(decoded.Snapshot.BuildContextDigest).To(Equal("build-ctx-1"))
			Expect(decoded.Snapshot.SelectedRoots).To(Equal([]string{".", "services/a", "services/b"}))
			Expect(decoded.Files[0].BlobHash).To(Equal("blob-a"))
			Expect(decoded.Files[0].ContentHash).To(Equal("content-a"))
		})
	})

	// F-005: freeze the #208 coverage wire shape (phase/complete/counts/budgets/
	// diagnostics maps) including empty-omission and non-null behavior.
	When("Coverage is empty or populated", func() {
		It("freezes exact keys, map types, empty-array omission, and stable ordering", func() {
			empty := baseModel()
			emptyJSON, err := json.Marshal(empty)
			Expect(err).NotTo(HaveOccurred())
			var emptyDoc map[string]json.RawMessage
			Expect(json.Unmarshal(emptyJSON, &emptyDoc)).To(Succeed())
			var emptyCov map[string]json.RawMessage
			Expect(json.Unmarshal(emptyDoc["coverage"], &emptyCov)).To(Succeed())
			Expect(emptyCov).To(HaveKey("phase"))
			Expect(emptyCov).To(HaveKey("complete"))
			Expect(emptyCov).NotTo(HaveKey("counts"))
			Expect(emptyCov).NotTo(HaveKey("budgets"))
			Expect(emptyCov).NotTo(HaveKey("diagnostics"))
			Expect(string(emptyDoc["coverage"])).NotTo(ContainSubstring("null"))

			populated := baseModel()
			populated.Coverage = projectmodel.Coverage{
				Phase:    "workspace_discovery",
				Complete: false,
				Counts:   map[string]int{"packages": 2, "files": 4},
				Budgets:  map[string]int{"files": 100, "edges": 50},
				Diagnostics: []projectmodel.Diagnostic{
					{Code: "z", Message: "z-msg", Path: "z.go"},
					{Code: "a", Message: "a-msg", Path: "a.go"},
				},
			}
			popJSON, err := json.Marshal(populated)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(popJSON)).To(ContainSubstring(`"counts":{"files":4,"packages":2}`))
			Expect(string(popJSON)).To(ContainSubstring(`"budgets":{"edges":50,"files":100}`))
			Expect(string(popJSON)).To(ContainSubstring(`"code":"a"`))
			// Diagnostics are sorted by code/path/message in canonical marshal.
			idxA := strings.Index(string(popJSON), `"code":"a"`)
			idxZ := strings.Index(string(popJSON), `"code":"z"`)
			Expect(idxA).To(BeNumerically("<", idxZ))
			Expect(string(popJSON)).NotTo(ContainSubstring("null"))
		})
	})

	When("optional fields are left at their zero value", func() {
		It("omits every omitempty key from the marshaled output", func() {
			model := baseModel()

			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			text := string(out)

			for _, key := range []string{
				`"repository"`, `"workspaces"`, `"modules"`, `"packages"`,
				`"files"`, `"import_edges"`, `"call_facts"`, `"reachability_facts"`,
			} {
				Expect(text).NotTo(ContainSubstring(key), "expected %s to be omitted from %s", key, text)
			}

			var top map[string]json.RawMessage
			Expect(json.Unmarshal(out, &top)).To(Succeed())
			var coverage map[string]json.RawMessage
			Expect(json.Unmarshal(top["coverage"], &coverage)).To(Succeed())
			Expect(coverage).NotTo(HaveKey("diagnostics"), "expected Coverage.Diagnostics to be omitted from %s", coverage)
		})

		It("omits nested omitempty keys when optional struct fields are zero", func() {
			model := baseModel()
			model.Workspaces = []projectmodel.Workspace{{ID: "root", Root: ".", Language: "go"}}
			model.ImportEdges = []projectmodel.ImportEdge{{From: "a", To: "b", Kind: "import"}}
			model.Coverage.Diagnostics = []projectmodel.Diagnostic{
				{Code: "unresolved_import", Message: "could not resolve module"},
			}

			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())

			var top map[string]json.RawMessage
			Expect(json.Unmarshal(out, &top)).To(Succeed())

			var snapshot map[string]json.RawMessage
			Expect(json.Unmarshal(top["snapshot"], &snapshot)).To(Succeed())
			Expect(snapshot).To(HaveKey("revision"), "expected Snapshot.Revision to be present in %s", snapshot)
			Expect(snapshot).To(HaveKey("tree_id"), "expected Snapshot.TreeID to be present in %s", snapshot)
			Expect(snapshot).To(HaveKey("config_digest"), "expected Snapshot.ConfigDigest to be present in %s", snapshot)
			Expect(snapshot).To(HaveKey("backend_digest"), "expected Snapshot.BackendDigest to be present in %s", snapshot)

			var workspaces []map[string]json.RawMessage
			Expect(json.Unmarshal(top["workspaces"], &workspaces)).To(Succeed())
			Expect(workspaces).To(HaveLen(1))
			Expect(workspaces[0]).NotTo(HaveKey("projects"), "expected Workspace.Projects to be omitted from %s", workspaces[0])

			var importEdges []map[string]json.RawMessage
			Expect(json.Unmarshal(top["import_edges"], &importEdges)).To(Succeed())
			Expect(importEdges).To(HaveLen(1))
			Expect(importEdges[0]).NotTo(HaveKey("site"), "expected ImportEdge.Site to be omitted from %s", importEdges[0])
			Expect(importEdges[0]).NotTo(HaveKey("resolution"), "expected ImportEdge.Resolution to be omitted from %s", importEdges[0])

			var coverage map[string]json.RawMessage
			Expect(json.Unmarshal(top["coverage"], &coverage)).To(Succeed())
			Expect(coverage).NotTo(HaveKey("counts"), "expected Coverage.Counts to be omitted from %s", coverage)
			Expect(coverage).NotTo(HaveKey("budgets"), "expected Coverage.Budgets to be omitted from %s", coverage)

			var diagnostics []map[string]json.RawMessage
			Expect(json.Unmarshal(coverage["diagnostics"], &diagnostics)).To(Succeed())
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0]).NotTo(HaveKey("path"), "expected Diagnostic.Path to be omitted from %s", diagnostics[0])
		})
	})

	When("CallFacts collection was not selected", func() {
		It("omits call_facts entirely (nil slice)", func() {
			model := baseModel()
			model.CallFacts = nil

			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).NotTo(ContainSubstring(`"call_facts"`))
		})
	})

	When("CallFacts collection was selected but found no facts", func() {
		It("serializes call_facts as an explicit empty array", func() {
			model := baseModel()
			model.CallFacts = []projectmodel.CallFact{}

			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).To(ContainSubstring(`"call_facts":[]`))
		})
	})

	When("call_facts is explicitly present as JSON null", func() {
		It("decodes to a nil CallFacts slice, matching an absent key", func() {
			model := baseModel()
			model.CallFacts = nil
			absent, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())

			var withNull map[string]any
			Expect(json.Unmarshal(absent, &withNull)).To(Succeed())
			withNull["call_facts"] = nil
			raw, err := json.Marshal(withNull)
			Expect(err).NotTo(HaveOccurred())

			var decoded projectmodel.Model
			Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
			Expect(decoded.CallFacts).To(BeNil())

			roundTrip, err := json.Marshal(decoded)
			Expect(err).NotTo(HaveOccurred())
			Expect(roundTrip).To(Equal(absent))
		})
	})

	When("call_facts is decoded in both directions", func() {
		It("decodes a present empty array to a non-nil, zero-length slice", func() {
			model := baseModel()
			model.CallFacts = []projectmodel.CallFact{}
			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())

			var decoded projectmodel.Model
			Expect(json.Unmarshal(out, &decoded)).To(Succeed())
			Expect(decoded.CallFacts).NotTo(BeNil())
			Expect(decoded.CallFacts).To(HaveLen(0))

			roundTrip, err := json.Marshal(decoded)
			Expect(err).NotTo(HaveOccurred())
			Expect(roundTrip).To(Equal(out))
		})

		It("decodes an absent key to a nil slice", func() {
			model := baseModel()
			model.CallFacts = nil
			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).NotTo(ContainSubstring(`"call_facts"`))

			var decoded projectmodel.Model
			Expect(json.Unmarshal(out, &decoded)).To(Succeed())
			Expect(decoded.CallFacts).To(BeNil())

			roundTrip, err := json.Marshal(decoded)
			Expect(err).NotTo(HaveOccurred())
			Expect(roundTrip).To(Equal(out))
		})
	})

	When("ReachabilityFacts collection was not selected", func() {
		It("omits reachability_facts entirely (nil slice)", func() {
			model := baseModel()
			model.ReachabilityFacts = nil

			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).NotTo(ContainSubstring(`"reachability_facts"`))
		})
	})

	When("ReachabilityFacts collection was selected but found no facts", func() {
		It("serializes reachability_facts as an explicit empty array", func() {
			model := baseModel()
			model.ReachabilityFacts = []projectmodel.ReachabilityFact{}

			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).To(ContainSubstring(`"reachability_facts":[]`))
		})
	})

	When("reachability_facts is explicitly present as JSON null", func() {
		It("decodes to a nil ReachabilityFacts slice, matching an absent key", func() {
			model := baseModel()
			model.ReachabilityFacts = nil
			absent, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())

			var withNull map[string]any
			Expect(json.Unmarshal(absent, &withNull)).To(Succeed())
			withNull["reachability_facts"] = nil
			raw, err := json.Marshal(withNull)
			Expect(err).NotTo(HaveOccurred())

			var decoded projectmodel.Model
			Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
			Expect(decoded.ReachabilityFacts).To(BeNil())

			roundTrip, err := json.Marshal(decoded)
			Expect(err).NotTo(HaveOccurred())
			Expect(roundTrip).To(Equal(absent))
		})
	})

	When("reachability_facts is decoded in both directions", func() {
		It("decodes a present empty array to a non-nil, zero-length slice", func() {
			model := baseModel()
			model.ReachabilityFacts = []projectmodel.ReachabilityFact{}
			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())

			var decoded projectmodel.Model
			Expect(json.Unmarshal(out, &decoded)).To(Succeed())
			Expect(decoded.ReachabilityFacts).NotTo(BeNil())
			Expect(decoded.ReachabilityFacts).To(HaveLen(0))

			roundTrip, err := json.Marshal(decoded)
			Expect(err).NotTo(HaveOccurred())
			Expect(roundTrip).To(Equal(out))
		})

		It("decodes an absent key to a nil slice", func() {
			model := baseModel()
			model.ReachabilityFacts = nil
			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).NotTo(ContainSubstring(`"reachability_facts"`))

			var decoded projectmodel.Model
			Expect(json.Unmarshal(out, &decoded)).To(Succeed())
			Expect(decoded.ReachabilityFacts).To(BeNil())

			roundTrip, err := json.Marshal(decoded)
			Expect(err).NotTo(HaveOccurred())
			Expect(roundTrip).To(Equal(out))
		})

		It("round-trips a populated fact unchanged", func() {
			model := baseModel()
			model.ReachabilityFacts = []projectmodel.ReachabilityFact{{
				ID:         "reach:A->B@algo@1",
				Kind:       projectmodel.KindPossibleCallReachability,
				Confidence: projectmodel.ReachabilityConfidenceResolvedDirect,
				Source:     "A",
				Sink:       "B",
				Path: []projectmodel.ReachabilityStep{
					{NodeID: "A"},
					{NodeID: "B"},
				},
				AlgorithmVersion: "algo@1",
			}}

			out, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())

			var decoded projectmodel.Model
			Expect(json.Unmarshal(out, &decoded)).To(Succeed())
			Expect(decoded.ReachabilityFacts).To(Equal(model.ReachabilityFacts))
		})
	})

	When("Coverage and Diagnostic are marshaled on their own", func() {
		It("round-trips a fact-only, codesignal-free diagnostics contract", func() {
			coverage := projectmodel.Coverage{
				Phase:    "import_resolution",
				Complete: false,
				Counts:   map[string]int{"files": 3},
				Budgets:  map[string]int{"files": 10},
				Diagnostics: []projectmodel.Diagnostic{
					{Code: "unresolved_import", Message: "could not resolve module", Path: "pkg/foo/foo.go"},
				},
			}

			out, err := json.Marshal(coverage)
			Expect(err).NotTo(HaveOccurred())

			var decoded projectmodel.Coverage
			Expect(json.Unmarshal(out, &decoded)).To(Succeed())
			Expect(decoded).To(Equal(coverage))
			Expect(decoded.Diagnostics[0].Code).To(Equal("unresolved_import"))
			Expect(decoded.Diagnostics[0].Path).To(Equal("pkg/foo/foo.go"))
		})
	})

	When("the same Model is marshaled twice", func() {
		It("produces byte-identical output both times", func() {
			model := baseModel()
			model.Workspaces = []projectmodel.Workspace{{ID: "root", Root: ".", Language: "go", Projects: []string{"module:."}}}
			model.ImportEdges = []projectmodel.ImportEdge{{From: "a", To: "b", Kind: "import"}}
			model.Coverage.Counts = map[string]int{"zebra": 1, "apple": 2, "mango": 3}
			model.Coverage.Budgets = map[string]int{"zulu": 9, "alpha": 1, "middle": 5}

			first, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			second, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			Expect(first).To(Equal(second))
		})
	})

	When("semantically identical models are assembled in different slice orders", func() {
		It("marshals to the same canonical bytes", func() {
			left := baseModel()
			left.Workspaces = []projectmodel.Workspace{
				{ID: "workspace:b", Language: "go", Root: "b", Projects: []string{"module:b2", "module:b1"}},
				{ID: "workspace:a", Language: "go", Root: "a", Projects: []string{"module:a"}},
			}
			left.Modules = []projectmodel.Module{
				{ID: "module:b", Path: "b", Language: "go", Files: []string{"b/z.go", "b/a.go"}},
				{ID: "module:a", Path: "a", Language: "go", Files: []string{"a/a.go"}},
			}
			left.Packages = []projectmodel.Package{
				{ID: "package:b", Path: "b", Language: "go", Files: []string{"b/z.go", "b/a.go"}},
				{ID: "package:a", Path: "a", Language: "go", Files: []string{"a/a.go"}},
			}
			left.Files = []projectmodel.File{
				{ID: "file:b/z.go", Path: "b/z.go", Language: "go"},
				{ID: "file:a/a.go", Path: "a/a.go", Language: "go"},
			}
			left.ImportEdges = []projectmodel.ImportEdge{
				{From: "b", To: "a", Kind: "import", Site: "b/z.go:1"},
				{From: "a", To: "b", Kind: "import", Site: "a/a.go:1"},
			}
			left.CallFacts = []projectmodel.CallFact{
				{From: "B", To: "A"},
				{From: "A", To: "B"},
			}
			left.ReachabilityFacts = []projectmodel.ReachabilityFact{
				{ID: "reach:B->A@algo@1", Kind: projectmodel.KindPossibleCallReachability, Confidence: projectmodel.ReachabilityConfidenceResolvedDirect, Source: "B", Sink: "A", Path: []projectmodel.ReachabilityStep{{NodeID: "B"}, {NodeID: "A"}}, AlgorithmVersion: "algo@1"},
				{ID: "reach:A->B@algo@1", Kind: projectmodel.KindPossibleCallReachability, Confidence: projectmodel.ReachabilityConfidenceResolvedDirect, Source: "A", Sink: "B", Path: []projectmodel.ReachabilityStep{{NodeID: "A"}, {NodeID: "B"}}, AlgorithmVersion: "algo@1"},
			}
			left.Coverage.Diagnostics = []projectmodel.Diagnostic{
				{Code: "unresolved_import", Message: "b", Path: "b/z.go"},
				{Code: "unresolved_import", Message: "a", Path: "a/a.go"},
			}

			right := baseModel()
			right.Workspaces = []projectmodel.Workspace{
				{ID: "workspace:a", Language: "go", Root: "a", Projects: []string{"module:a"}},
				{ID: "workspace:b", Language: "go", Root: "b", Projects: []string{"module:b1", "module:b2"}},
			}
			right.Modules = []projectmodel.Module{
				{ID: "module:a", Path: "a", Language: "go", Files: []string{"a/a.go"}},
				{ID: "module:b", Path: "b", Language: "go", Files: []string{"b/a.go", "b/z.go"}},
			}
			right.Packages = []projectmodel.Package{
				{ID: "package:a", Path: "a", Language: "go", Files: []string{"a/a.go"}},
				{ID: "package:b", Path: "b", Language: "go", Files: []string{"b/a.go", "b/z.go"}},
			}
			right.Files = []projectmodel.File{
				{ID: "file:a/a.go", Path: "a/a.go", Language: "go"},
				{ID: "file:b/z.go", Path: "b/z.go", Language: "go"},
			}
			right.ImportEdges = []projectmodel.ImportEdge{
				{From: "a", To: "b", Kind: "import", Site: "a/a.go:1"},
				{From: "b", To: "a", Kind: "import", Site: "b/z.go:1"},
			}
			right.CallFacts = []projectmodel.CallFact{
				{From: "A", To: "B"},
				{From: "B", To: "A"},
			}
			right.ReachabilityFacts = []projectmodel.ReachabilityFact{
				{ID: "reach:A->B@algo@1", Kind: projectmodel.KindPossibleCallReachability, Confidence: projectmodel.ReachabilityConfidenceResolvedDirect, Source: "A", Sink: "B", Path: []projectmodel.ReachabilityStep{{NodeID: "A"}, {NodeID: "B"}}, AlgorithmVersion: "algo@1"},
				{ID: "reach:B->A@algo@1", Kind: projectmodel.KindPossibleCallReachability, Confidence: projectmodel.ReachabilityConfidenceResolvedDirect, Source: "B", Sink: "A", Path: []projectmodel.ReachabilityStep{{NodeID: "B"}, {NodeID: "A"}}, AlgorithmVersion: "algo@1"},
			}
			right.Coverage.Diagnostics = []projectmodel.Diagnostic{
				{Code: "unresolved_import", Message: "a", Path: "a/a.go"},
				{Code: "unresolved_import", Message: "b", Path: "b/z.go"},
			}

			leftJSON, err := json.Marshal(left)
			Expect(err).NotTo(HaveOccurred())
			rightJSON, err := json.Marshal(right)
			Expect(err).NotTo(HaveOccurred())
			Expect(leftJSON).To(Equal(rightJSON))
		})
	})
})
