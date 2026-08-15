package projectmodel_test

import (
	"os"
	"sort"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func diagnosticCodes(diags []projectmodel.Diagnostic) []string {
	codes := make([]string, 0, len(diags))
	for _, d := range diags {
		codes = append(codes, d.Code)
	}
	sort.Strings(codes)
	return codes
}

func hasDiagnostic(diags []projectmodel.Diagnostic, code, path string) bool {
	for _, d := range diags {
		if d.Code == code && d.Path == path {
			return true
		}
	}
	return false
}

var _ = Describe("DiscoverGoRoots", func() {
	When("snapshot has no go.mod/go.work anywhere", func() {
		It("returns an empty, complete result with the frozen coverage shape", func() {
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
		})
	})

	When("snapshot has multiple nested go.mod roots with no go.work", func() {
		It("discovers every distinct module root, sorted deterministically", func() {
			snapshot := os.DirFS("testdata/go_roots_nested")
			result, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Complete).To(BeTrue())
			Expect(result.Roots).To(Equal([]string{"a", "b/sub"}))
			Expect(result.Coverage.Counts).To(Equal(map[string]int{
				"files_seen":      2,
				"files_skipped":   0,
				"modules_seen":    2,
				"modules_skipped": 0,
				"roots_emitted":   2,
			}))
		})
	})

	When("a go.work use block claims the same directory twice", func() {
		It("emits a project_root_duplicate diagnostic and still emits the root once", func() {
			snapshot := os.DirFS("testdata/go_roots_duplicate")
			result, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Roots).To(ContainElement("dup"))
			Expect(hasDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagRootDuplicate, "dup")).To(BeTrue(),
				"expected a project_root_duplicate diagnostic for path \"dup\", got %+v", result.Coverage.Diagnostics)
		})
	})

	When("a discovered go.mod is unparseable", func() {
		It("emits a project_root_invalid diagnostic and excludes the root", func() {
			snapshot := os.DirFS("testdata/go_roots_invalid")
			result, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Roots).NotTo(ContainElement("bad"))
			Expect(hasDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagRootInvalid, "bad/go.mod")).To(BeTrue(),
				"expected a project_root_invalid diagnostic for path \"bad/go.mod\", got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Counts).To(HaveKeyWithValue("modules_skipped", 1))
		})
	})

	When("a go.work use directive escapes the snapshot root", func() {
		It("emits a project_root_outside_snapshot diagnostic and excludes the root", func() {
			snapshot := os.DirFS("testdata/go_roots_escaping")
			result, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Roots).To(BeEmpty())
			Expect(hasDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagRootOutsideSnapshot, "../outside")).To(BeTrue(),
				"expected a project_root_outside_snapshot diagnostic for path \"../outside\", got %+v", result.Coverage.Diagnostics)
		})
	})

	When("a use-referenced directory is itself claimed as a nested workspace root", func() {
		It("emits a project_root_ambiguous diagnostic", func() {
			snapshot := os.DirFS("testdata/go_roots_ambiguous")
			result, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			Expect(hasDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagRootAmbiguous, "nested")).To(BeTrue(),
				"expected a project_root_ambiguous diagnostic for path \"nested\", got %+v", result.Coverage.Diagnostics)
			Expect(result.Roots).To(ContainElement("nested"))
			Expect(result.Roots).To(ContainElement("nested/inner"))
		})
	})

	When("the snapshot filesystem itself cannot be read", func() {
		It("emits a project_root_unavailable diagnostic instead of returning an error", func() {
			snapshot := os.DirFS("testdata/go_roots_missing_on_purpose")
			result, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Complete).To(BeFalse())
			Expect(result.Roots).To(BeEmpty())
			Expect(hasDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagRootUnavailable, ".")).To(BeTrue(),
				"expected a project_root_unavailable diagnostic, got %+v", result.Coverage.Diagnostics)
		})
	})

	When("a budget is exceeded before discovery finishes walking", func() {
		It("truncates deterministically, marks Complete false, and emits project_root_incomplete", func() {
			snapshot := os.DirFS("testdata/go_roots_incomplete")
			result, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{MaxInputFiles: 1})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Complete).To(BeFalse())
			Expect(hasDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagRootIncomplete, "")).To(BeTrue(),
				"expected a project_root_incomplete diagnostic, got %+v", result.Coverage.Diagnostics)
			Expect(len(result.Roots)).To(BeNumerically("<", 3))
			Expect(result.Coverage.Counts).To(HaveKeyWithValue("files_skipped", 1))

			again, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{MaxInputFiles: 1})
			Expect(err).NotTo(HaveOccurred())
			Expect(again.Roots).To(Equal(result.Roots), "truncation must be deterministic across repeated calls")
		})
	})

	When("the go_multiworkspace fixture is discovered", func() {
		It("finds the workspace root and both module roots", func() {
			snapshot := os.DirFS("testdata/go_multiworkspace")
			result, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Complete).To(BeTrue())
			Expect(result.Roots).To(Equal([]string{".", "modulea", "moduleb"}))
			Expect(result.Coverage.Diagnostics).To(BeEmpty())
		})
	})

	When("a go.mod sits under a testdata/, vendor/, or dot-prefixed subdirectory of a larger tree", func() {
		It("never treats those nested go.mod files as roots, matching the go tool's own convention", func() {
			snapshot := os.DirFS("testdata/go_roots_excluded_dirs")
			result, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Complete).To(BeTrue())
			Expect(result.Roots).To(Equal([]string{"."}),
				"testdata/, vendor/, and .hidden/ go.mod fixtures must be pruned, not reported as roots")
		})
	})

	When("a build produces diagnostics with two different codes", func() {
		It("returns Coverage.Diagnostics sorted by code then path, not walk/resolution order", func() {
			snapshot := fstest.MapFS{
				"bad/go.mod": &fstest.MapFile{Data: []byte("this is not a valid go.mod {{{")},
				"dup/go.mod": &fstest.MapFile{Data: []byte("module example.com/dup\n\ngo 1.25\n")},
				"go.work":    &fstest.MapFile{Data: []byte("go 1.25\n\nuse (\n\t./dup\n\tdup\n)\n")},
			}
			result, err := projectmodel.DiscoverGoRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())

			// Walk order discovers bad/go.mod (project_root_invalid) before
			// resolveUseDirectives emits project_root_duplicate for "dup";
			// "duplicate" < "invalid" lexically, so an unsorted result would
			// have these reversed.
			Expect(result.Coverage.Diagnostics).To(HaveLen(2))
			Expect(result.Coverage.Diagnostics[0].Code).To(Equal(projectmodel.DiagRootDuplicate))
			Expect(result.Coverage.Diagnostics[0].Path).To(Equal("dup"))
			Expect(result.Coverage.Diagnostics[1].Code).To(Equal(projectmodel.DiagRootInvalid))
			Expect(result.Coverage.Diagnostics[1].Path).To(Equal("bad/go.mod"))
		})
	})
})
