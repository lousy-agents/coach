package projectmodel_test

import (
	"os"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_goWorkspaceAcceptanceTest_returnsAnEmptyCompleteResultWithTheFrozenCoverag_34() {
	snapshot := os.DirFS("testdata/go_roots_empty")
	result, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{})
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Roots).To(BeEmpty())
	Expect(result.Complete).To(BeTrue())
	Expect(result.Coverage.Diagnostics).To(BeEmpty())
	Expect(result.Coverage.Counts).To(Equal(map[string]int{
		"files_seen":      1, // testdata/go_roots_empty/README.md
		"files_skipped":   0,
		"modules_seen":    0,
		"modules_skipped": 0,
		"roots_emitted":   0,
	}))
	for _, key := range []string{"wall_time_ms", "input_files", "input_bytes", "graph_nodes", "graph_edges", "working_set_bytes", "stderr_bytes"} {
		Expect(result.Coverage.Budgets).To(HaveKey(key), "expected effective budget key %q", key)
	}
	Expect(result.Coverage.Budgets).To(HaveKeyWithValue("stderr_bytes", 0))
}
