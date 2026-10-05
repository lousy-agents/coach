package projectmodel_test

import (
	"os"
	"testing/fstest"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_tsWorkspaceAcceptanceTest_returnsAnEmptyCompleteResultWithTheFrozenCoverag_15() {
	snapshot := fstest.MapFS{
		"README.md": &fstest.MapFile{Data: []byte("nothing here")},
	}
	result, err := projectmodel.DiscoverTSRoots(snapshot, projectmodel.GoBudgets{})
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Roots).To(BeEmpty())
	Expect(result.Candidates).To(BeEmpty())
	Expect(result.Complete).To(BeTrue())
	Expect(result.Coverage.Diagnostics).To(BeEmpty())
	for _, key := range []string{"wall_time_ms", "input_files", "input_bytes", "graph_nodes", "graph_edges", "working_set_bytes", "stderr_bytes"} {
		Expect(result.Coverage.Budgets).To(HaveKey(key), "expected effective budget key %q", key)
	}
}

func body_tsWorkspaceAcceptanceTest_emitsATsProjectRootUnavailableDiagnosticInsteadO_74() {
	// This directory is intentionally absent, mirroring
	// go_workspace_acceptance_test.go's "testdata/go_roots_missing_on_purpose"
	// precedent -- os.DirFS is lazy, so the failure surfaces only when
	// WalkDir tries to read ".".
	snapshot := os.DirFS("testdata/ts_roots_missing_on_purpose")
	result, err := projectmodel.DiscoverTSRoots(snapshot, projectmodel.GoBudgets{})
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Complete).To(BeFalse())
	Expect(result.Roots).To(BeEmpty())
	Expect(result.Candidates).To(BeEmpty())
	found := false
	for _, diag := range result.Coverage.Diagnostics {
		if diag.Code == projectmodel.DiagTSRootUnavailable && diag.Path == "." {
			found = true
		}
	}
	Expect(found).To(BeTrue(), "expected a ts_project_root_unavailable diagnostic, got %+v", result.Coverage.Diagnostics)
}

func body_tsWorkspaceAcceptanceTest_truncatesDeterministicallyMarksCompleteFalseAndE_96() {
	snapshot := fstest.MapFS{
		"a/tsconfig.json": &fstest.MapFile{Data: []byte(`{}`)},
		"b/tsconfig.json": &fstest.MapFile{Data: []byte(`{}`)},
		"c/tsconfig.json": &fstest.MapFile{Data: []byte(`{}`)},
	}
	result, err := projectmodel.DiscoverTSRoots(snapshot, projectmodel.GoBudgets{MaxInputFiles: 1})
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Complete).To(BeFalse())
	found := false
	for _, diag := range result.Coverage.Diagnostics {
		if diag.Code == projectmodel.DiagTSRootIncomplete {
			found = true
		}
	}
	Expect(found).To(BeTrue(), "expected a ts_project_root_incomplete diagnostic, got %+v", result.Coverage.Diagnostics)
	Expect(len(result.Roots)).To(BeNumerically("<", 3))

	again, err := projectmodel.DiscoverTSRoots(snapshot, projectmodel.GoBudgets{MaxInputFiles: 1})
	Expect(err).NotTo(HaveOccurred())
	Expect(again.Roots).To(Equal(result.Roots), "truncation must be deterministic across repeated calls")
}

func body_tsWorkspaceAcceptanceTest_truncatesMarksCompleteFalseAndEmitsTsProjectRoot_135() {
	snapshot := fstest.MapFS{
		"apps/api/tsconfig.json": &fstest.MapFile{Data: make([]byte, 64)},
		"apps/web/tsconfig.json": &fstest.MapFile{Data: []byte(`{}`)},
	}
	result, err := projectmodel.DiscoverTSRoots(snapshot, projectmodel.GoBudgets{MaxInputBytes: 32})
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Complete).To(BeFalse())
	found := false
	for _, diag := range result.Coverage.Diagnostics {
		if diag.Code == projectmodel.DiagTSRootIncomplete {
			found = true
		}
	}
	Expect(found).To(BeTrue(), "expected a ts_project_root_incomplete diagnostic, got %+v", result.Coverage.Diagnostics)
	Expect(len(result.Roots) + len(result.Candidates)).To(BeNumerically("<", 2))
}
