package projectmodel_test

import (
	"context"
	"os"
	"reflect"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_goReachabilityAcceptanceTest_carriesNoSeverityLifecycleOrActiveFindingShapedF_50() {
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
}

func body_goReachabilityAcceptanceTest_carriesNoSeverityLifecycleOrActiveFindingShapedF_87() {
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
}

func body_goReachabilityAcceptanceTest_treatsEveryPairAsUnevaluatedRatherThanReportingA_184() {
	snapshot := os.DirFS("testdata/go_reachability_path")
	result, err := projectmodel.BuildGoReachability(context.Background(), snapshot, projectmodel.ReachabilityOptions{
		Budgets: projectmodel.GoBudgets{MaxGraphNodes: 1},
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(result.Coverage.Complete).To(BeFalse())
	Expect(result.Coverage.Counts["source_sink_pairs_evaluated"]).To(Equal(0),
		"a pair searched against an incompletely built call graph must not count as conclusively evaluated")
	Expect(result.Coverage.Counts["source_sink_pairs_truncated"]).To(BeNumerically(">", 0))

	Expect(result.Coverage.Budgets).To(HaveKeyWithValue("graph_nodes", 1))
	for _, key := range []string{"wall_time_ms", "input_files", "input_bytes", "graph_nodes", "graph_edges", "working_set_bytes", "stderr_bytes", "search_nodes"} {
		Expect(result.Coverage.Budgets).To(HaveKey(key), "expected effective budget key %q", key)
	}
}
