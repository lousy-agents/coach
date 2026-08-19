package projectmodel_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func hasReachabilityDiagnostic(diags []projectmodel.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func countReachabilityDiagnostic(diags []projectmodel.Diagnostic, code string) int {
	n := 0
	for _, d := range diags {
		if d.Code == code {
			n++
		}
	}
	return n
}

var _ = Describe("BuildGoReachability", func() {
	When("a source function calls through two intermediate functions to a pinned database sink", func() {
		It("produces a ReachabilityFact with a stable ID, a populated Confidence, and an ordered Path", func() {
			snapshot := os.DirFS("testdata/go_reachability_path")
			result, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Facts).To(HaveLen(1), "expected exactly one reachability fact, got %+v", result.Facts)
			fact := result.Facts[0]

			Expect(fact.ID).NotTo(BeEmpty())
			Expect(fact.Confidence).NotTo(BeEmpty())
			Expect(fact.Kind).To(Equal(projectmodel.KindPossibleCallReachability))
			Expect(fact.AlgorithmVersion).To(Equal(projectmodel.ReachabilityAlgorithm))

			Expect(fact.Path).To(HaveLen(4), "expected source, two intermediates, and the sink, got %+v", fact.Path)
			Expect(fact.Path[0].NodeID).To(Equal("example.com/reachabilitypath.Handler"))
			Expect(fact.Path[1].NodeID).To(Equal("example.com/reachabilitypath.loadUser"))
			Expect(fact.Path[2].NodeID).To(Equal("example.com/reachabilitypath.queryDB"))
			Expect(fact.Path[3].NodeID).To(Equal("(*database/sql.DB).Query"))

			Expect(result.Coverage.Complete).To(BeTrue())
			Expect(result.Coverage.Counts).To(HaveKeyWithValue("ssa_programs_built", 1),
				"call-graph and source identification must share one SSA program per root, got counts=%v", result.Coverage.Counts)

			// Running twice, from a fresh call, must reproduce the same ID.
			second, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(second.Facts).To(HaveLen(1))
			Expect(second.Facts[0].ID).To(Equal(fact.ID))
		})
	})

	// Negative control: the issue's core structural requirement is that no
	// active dataflow.source_reaches_sink Signal or severity/lifecycle
	// concept ever attaches to a reachability observation. Enforce this by
	// reflection over the type itself, not just documentation, so a field
	// added back later fails this test.
	Describe("the facts-only structural contract", func() {
		It("carries no severity, lifecycle, or active-finding-shaped field anywhere on ReachabilityFact", func() {
			allowed := map[string]bool{
				"ID":               true,
				"Kind":             true,
				"Confidence":       true,
				"Source":           true,
				"Sink":             true,
				"Path":             true,
				"AlgorithmVersion": true,
			}
			forbiddenSubstrings := []string{"severity", "lifecycle", "changed", "active", "finding", "status"}

			factType := reflect.TypeOf(projectmodel.ReachabilityFact{})
			seen := map[string]bool{}
			for i := 0; i < factType.NumField(); i++ {
				name := factType.Field(i).Name
				seen[name] = true
				Expect(allowed).To(HaveKey(name), "ReachabilityFact gained an unexpected field %q not on the reviewed allowlist", name)
				lower := strings.ToLower(name)
				for _, bad := range forbiddenSubstrings {
					Expect(strings.Contains(lower, bad)).To(BeFalse(), "ReachabilityFact field %q looks severity/lifecycle/active-finding-shaped", name)
				}
			}
			for name := range allowed {
				Expect(seen).To(HaveKey(name), "expected allowlisted field %q to still exist on ReachabilityFact", name)
			}

			stepType := reflect.TypeOf(projectmodel.ReachabilityStep{})
			for i := 0; i < stepType.NumField(); i++ {
				name := stepType.Field(i).Name
				lower := strings.ToLower(name)
				for _, bad := range forbiddenSubstrings {
					Expect(strings.Contains(lower, bad)).To(BeFalse(), "ReachabilityStep field %q looks severity/lifecycle/active-finding-shaped", name)
				}
			}
		})

		It("carries no severity, lifecycle, or active-finding-shaped field anywhere on ReachabilityResult", func() {
			allowedResultFields := map[string]bool{
				"Facts":     true,
				"Sources":   true,
				"Algorithm": true,
				"Coverage":  true,
			}
			forbiddenSubstrings := []string{"severity", "lifecycle", "changed", "active", "finding", "status"}

			resultType := reflect.TypeOf(projectmodel.ReachabilityResult{})
			seen := map[string]bool{}
			for i := 0; i < resultType.NumField(); i++ {
				name := resultType.Field(i).Name
				seen[name] = true
				Expect(allowedResultFields).To(HaveKey(name), "ReachabilityResult gained an unexpected field %q not on the reviewed allowlist", name)
				lower := strings.ToLower(name)
				for _, bad := range forbiddenSubstrings {
					Expect(strings.Contains(lower, bad)).To(BeFalse(), "ReachabilityResult field %q looks severity/lifecycle/active-finding-shaped", name)
				}
			}
			for name := range allowedResultFields {
				Expect(seen).To(HaveKey(name), "expected allowlisted field %q to still exist on ReachabilityResult", name)
			}
		})
	})

	When("no source has a call path to any sink within the full traversal", func() {
		It("produces no ReachabilityFact and marks the pair fully evaluated via Coverage.Complete", func() {
			snapshot := os.DirFS("testdata/go_reachability_no_path")
			result, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Facts).To(BeEmpty())
			Expect(result.Coverage.Complete).To(BeTrue(),
				"a fully searched no-path result must report Complete true, distinct from a truncated search")
			Expect(result.Coverage.Counts["source_sink_pairs_total"]).To(BeNumerically(">", 0))
			Expect(result.Coverage.Counts["source_sink_pairs_evaluated"]).To(Equal(result.Coverage.Counts["source_sink_pairs_total"]),
				"every source x sink pair must be conclusively evaluated when the search is not truncated")
			Expect(result.Coverage.Counts["source_sink_pairs_truncated"]).To(Equal(0))
		})
	})

	When("the search is interrupted by a node-visitation budget before completion", func() {
		It("marks Coverage.Complete false, records an explicit truncation diagnostic and count, and returns a bounded partial result", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_reachability_path")
			result, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{
				MaxSearchNodes: 1,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Coverage.Complete).To(BeFalse())
			Expect(hasReachabilityDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagReachabilityBudgetExceeded)).To(BeTrue(),
				"expected a project_reachability_budget_exceeded diagnostic, got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Counts["source_sink_pairs_truncated"]).To(BeNumerically(">", 0))
			Expect(len(result.Facts)).To(BeNumerically("<=", 1))
			Expect(result.Coverage.Budgets).To(HaveKeyWithValue("search_nodes", 1))
		}, SpecTimeout(20*time.Second))
	})

	When("a source's only call route to a pinned sink passes through a synthetic bound-method-value wrapper whose real target is local to the snapshot", func() {
		It("reports no ReachabilityFact for the pair and marks Coverage.Complete false with the call graph's synthetic-wrapper diagnostic, instead of silently dead-ending", func() {
			snapshot := os.DirFS("testdata/go_reachability_bound_method")
			result, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Facts).To(BeEmpty(),
				"a call route that dead-ends at a local-targeted synthetic wrapper must not silently synthesize a path through it")
			Expect(result.Coverage.Complete).To(BeFalse(),
				"the underlying call graph's synthetic-wrapper dead end must propagate as reachability incompleteness, not report a silently 'complete' no-path result")
			Expect(hasReachabilityDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedSyntheticWrapper)).To(BeTrue(),
				"expected the call graph's project_call_unresolved_synthetic_wrapper diagnostic to surface on ReachabilityResult.Coverage, got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Counts["source_sink_pairs_evaluated"]).To(Equal(0),
				"a pair searched against an incompletely built call graph must not count as conclusively evaluated")
			Expect(result.Coverage.Counts["source_sink_pairs_truncated"]).To(BeNumerically(">", 0))
			Expect(result.Coverage.Counts["underlying_unresolved_call_sites"]).To(Equal(1),
				"the call graph's unresolved_synthetic_wrapper count must propagate into ReachabilityResult's unresolved-ratio input, not be dropped to zero")
			Expect(hasReachabilityDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagReachabilityBudgetExceeded)).To(BeFalse(),
				"no budget was ever exceeded in this run (search_nodes and wall_time_ms are both unbounded); a call-graph dead end must not also report a false budget-exceeded diagnostic")
		})
	})

	When("a snapshot contains a resolvable source-to-sink route alongside an unrelated local generic function call", func() {
		It("still produces the resolvable ReachabilityFact instead of losing all facts under a false Coverage.Complete=false", func() {
			snapshot := os.DirFS("testdata/go_reachability_generic_call")
			result, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Facts).To(HaveLen(1),
				"expected the resolvable source-to-sink route to survive an unrelated local generic call, got %+v", result.Facts)
			Expect(result.Coverage.Complete).To(BeTrue(),
				"an unrelated local generic function call must not flip reachability Coverage.Complete false for the whole snapshot")
			Expect(result.Coverage.Counts["source_sink_pairs_evaluated"]).To(BeNumerically(">", 0),
				"a false call-graph incompleteness must not zero out source_sink_pairs_evaluated")
		})
	})

	When("the underlying call-graph build itself is truncated by a graph-node budget", func() {
		It("treats every pair as unevaluated rather than reporting a truncated call graph as a fully searched one", func() {
			snapshot := os.DirFS("testdata/go_reachability_path")
			result, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{
				Budgets: projectmodel.GoBudgets{MaxGraphNodes: 1},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Coverage.Complete).To(BeFalse())
			Expect(result.Coverage.Counts["source_sink_pairs_evaluated"]).To(Equal(0),
				"a pair searched against an incompletely built call graph must not count as conclusively evaluated")
			Expect(result.Coverage.Counts["source_sink_pairs_truncated"]).To(BeNumerically(">", 0))

			// The forwarded GoBudgets budget that actually truncated this run
			// (graph_nodes) must survive effectiveReachabilityBudgets
			// unmutated, alongside the full EffectiveGoBudgets vocabulary
			// plus reachability's own search_nodes key.
			Expect(result.Coverage.Budgets).To(HaveKeyWithValue("graph_nodes", 1))
			for _, key := range []string{"wall_time_ms", "input_files", "input_bytes", "graph_nodes", "graph_edges", "working_set_bytes", "stderr_bytes", "search_nodes"} {
				Expect(result.Coverage.Budgets).To(HaveKey(key), "expected effective budget key %q", key)
			}
		})
	})

	When("the context passed in is already cancelled before the search starts", func() {
		It("returns promptly with Complete false, a budget-exceeded diagnostic, and no facts", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_reachability_path")
			cancelledCtx, cancel := context.WithCancel(context.Background())
			cancel()

			result, err := projectmodel.BuildGoReachability(cancelledCtx, snapshot, projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Coverage.Complete).To(BeFalse())
			Expect(hasReachabilityDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagReachabilityBudgetExceeded)).To(BeTrue(),
				"expected a project_reachability_budget_exceeded diagnostic, got %+v", result.Coverage.Diagnostics)
			Expect(result.Facts).To(BeEmpty())
			Expect(countReachabilityDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagReachabilityBudgetExceeded)).To(Equal(1),
				"an already-cancelled ctx must not be reported as budget-exceeded twice, got %+v", result.Coverage.Diagnostics)
		}, SpecTimeout(20*time.Second))
	})

	When("a wall-time budget expires before the search can run", func() {
		It("returns promptly with Complete false, a budget-exceeded diagnostic, and no facts", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_reachability_path")
			result, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{
				Budgets: projectmodel.GoBudgets{WallTime: 1 * time.Nanosecond},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Coverage.Complete).To(BeFalse())
			Expect(hasReachabilityDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagReachabilityBudgetExceeded)).To(BeTrue(),
				"expected a project_reachability_budget_exceeded diagnostic, got %+v", result.Coverage.Diagnostics)
			Expect(result.Facts).To(BeEmpty())
			Expect(countReachabilityDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagReachabilityBudgetExceeded)).To(Equal(1),
				"an expired wall-time budget must not be reported as budget-exceeded twice, got %+v", result.Coverage.Diagnostics)
		}, SpecTimeout(20*time.Second))
	})

	Describe("coverage measurement fields", func() {
		It("populates path coverage, unresolved ratio inputs, runtime, memory, and truncation counts on a complete run", func() {
			snapshot := os.DirFS("testdata/go_reachability_path")
			result, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Coverage.Counts).To(HaveKey("source_sink_pairs_total"))
			Expect(result.Coverage.Counts).To(HaveKey("source_sink_pairs_evaluated"))
			Expect(result.Coverage.Counts).To(HaveKey("source_sink_pairs_truncated"))
			Expect(result.Coverage.Counts).To(HaveKey("underlying_call_sites_seen"))
			Expect(result.Coverage.Counts).To(HaveKey("underlying_unresolved_call_sites"))
			Expect(result.Coverage.Counts["runtime_ms"]).To(BeNumerically(">=", 0))
			Expect(result.Coverage.Counts["memory_bytes"]).To(BeNumerically(">=", 0))
		})

		It("populates the same coverage fields on a budget-truncated run", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_reachability_path")
			result, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{
				MaxSearchNodes: 1,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Coverage.Counts).To(HaveKey("source_sink_pairs_total"))
			Expect(result.Coverage.Counts).To(HaveKey("source_sink_pairs_evaluated"))
			Expect(result.Coverage.Counts).To(HaveKey("source_sink_pairs_truncated"))
			Expect(result.Coverage.Counts["runtime_ms"]).To(BeNumerically(">=", 0))
			Expect(result.Coverage.Counts["memory_bytes"]).To(BeNumerically(">=", 0))
		}, SpecTimeout(20*time.Second))
	})

	When("the same snapshot is analyzed twice, and from two different temp roots", func() {
		It("produces byte-identical ReachabilityFact ordering, IDs, and full marshaled Coverage", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_reachability_path")

			first, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())
			second, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())

			// Guard against a vacuous pass: json.Marshal(nil) == json.Marshal(nil)
			// would make this spec pass even if the traversal produced nothing.
			Expect(first.Facts).NotTo(BeEmpty())
			Expect(second.Facts).NotTo(BeEmpty())

			firstFactsJSON, err := json.Marshal(first.Facts)
			Expect(err).NotTo(HaveOccurred())
			secondFactsJSON, err := json.Marshal(second.Facts)
			Expect(err).NotTo(HaveOccurred())
			Expect(firstFactsJSON).To(Equal(secondFactsJSON))

			// Exact golden assertion: pins the wire key names themselves
			// (json.Marshal equality alone would still pass if node_id or
			// algorithm_version were renamed on both runs identically).
			Expect(string(firstFactsJSON)).To(Equal(`[{"id":"reach:example.com/reachabilitypath.Handler-\u003e(*database/sql.DB).Query@go-source-sink-registry@1","kind":"possible_call_reachability","confidence":"resolved_direct","source":"example.com/reachabilitypath.Handler","sink":"(*database/sql.DB).Query","path":[{"node_id":"example.com/reachabilitypath.Handler"},{"node_id":"example.com/reachabilitypath.loadUser"},{"node_id":"example.com/reachabilitypath.queryDB"},{"node_id":"(*database/sql.DB).Query"}],"algorithm_version":"go-source-sink-registry@1"}]`))

			left := copyFixtureToTempDir("testdata/go_reachability_path")
			right := copyFixtureToTempDir("testdata/go_reachability_path")
			Expect(left).NotTo(Equal(right))

			leftResult, err := projectmodel.BuildGoReachability(context.Background(), os.DirFS(left), projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())
			rightResult, err := projectmodel.BuildGoReachability(context.Background(), os.DirFS(right), projectmodel.ReachabilityOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(leftResult.Facts).To(HaveLen(1))
			Expect(rightResult.Facts).To(HaveLen(1))

			leftFactsJSON, err := json.Marshal(leftResult.Facts)
			Expect(err).NotTo(HaveOccurred())
			rightFactsJSON, err := json.Marshal(rightResult.Facts)
			Expect(err).NotTo(HaveOccurred())
			Expect(leftFactsJSON).To(Equal(rightFactsJSON))
			Expect(string(leftFactsJSON)).NotTo(ContainSubstring(left))
			Expect(string(leftFactsJSON)).NotTo(ContainSubstring(right))

			// runtime_ms/memory_bytes are wall-clock measurements and may
			// legitimately differ between two separate builds; everything
			// else in Coverage (including every other count and every
			// diagnostic) must still be byte-identical.
			leftCoverageJSON, err := json.Marshal(withoutWallClockCounts(leftResult.Coverage))
			Expect(err).NotTo(HaveOccurred())
			rightCoverageJSON, err := json.Marshal(withoutWallClockCounts(rightResult.Coverage))
			Expect(err).NotTo(HaveOccurred())
			Expect(leftCoverageJSON).To(Equal(rightCoverageJSON))
			Expect(string(leftCoverageJSON)).NotTo(ContainSubstring(left))
			Expect(string(leftCoverageJSON)).NotTo(ContainSubstring(right))
		}, SpecTimeout(20*time.Second))
	})
})

func withoutWallClockCounts(cov projectmodel.Coverage) projectmodel.Coverage {
	counts := make(map[string]int, len(cov.Counts))
	for k, v := range cov.Counts {
		if k == "runtime_ms" || k == "memory_bytes" {
			continue
		}
		counts[k] = v
	}
	cov.Counts = counts
	return cov
}
