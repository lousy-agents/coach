package projectmodel_test

import (
	"os"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var _ = Describe("DiscoverTSRoots", func() {
	When("snapshot has no tsconfig.json/package.json anywhere", func() {
		It("returns an empty, complete result with the frozen coverage shape", func() {
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
		})
	})

	When("snapshot has multiple nested tsconfig.json roots", func() {
		It("discovers every distinct root purely from the fs.FS snapshot, with no compiler or subprocess involved", func() {
			// This snapshot is a pure in-memory fstest.MapFS: there is no
			// real directory a subprocess could exec against, no node_modules,
			// and no tsc binary anywhere in reach. If discovery required a
			// compiler or subprocess it could not succeed here at all.
			snapshot := fstest.MapFS{
				"apps/api/tsconfig.json": &fstest.MapFile{Data: []byte(`{"compilerOptions":{}}`)},
				"apps/web/tsconfig.json": &fstest.MapFile{Data: []byte(`{"compilerOptions":{}}`)},
				"README.md":              &fstest.MapFile{Data: []byte("root readme")},
			}
			result, err := projectmodel.DiscoverTSRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Complete).To(BeTrue())
			Expect(result.Roots).To(Equal([]string{"apps/api", "apps/web"}))
			Expect(result.Coverage.Diagnostics).To(BeEmpty())
		})
	})

	When("a directory has a package.json but no tsconfig.json of its own", func() {
		It("reports it as a candidate, not a root, and does not group or label it by architectural layer", func() {
			snapshot := fstest.MapFS{
				"apps/frontend/package.json": &fstest.MapFile{Data: []byte(`{"name":"frontend"}`)},
				"apps/backend/package.json":  &fstest.MapFile{Data: []byte(`{"name":"backend"}`)},
				"apps/backend/tsconfig.json": &fstest.MapFile{Data: []byte(`{}`)},
			}
			result, err := projectmodel.DiscoverTSRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			// apps/backend has both files: it is a confirmed root, not a candidate.
			Expect(result.Roots).To(Equal([]string{"apps/backend"}))
			// apps/frontend has only a package.json: a bare candidate path,
			// never renamed, grouped, or labeled "frontend"/"ui"/any layer.
			Expect(result.Candidates).To(Equal([]string{"apps/frontend"}))
		})
	})

	When("a tsconfig.json sits inside node_modules or a dot-prefixed directory", func() {
		It("never treats it as a root or candidate", func() {
			snapshot := fstest.MapFS{
				"node_modules/some-lib/tsconfig.json": &fstest.MapFile{Data: []byte(`{}`)},
				".hidden/tsconfig.json":               &fstest.MapFile{Data: []byte(`{}`)},
				"real/tsconfig.json":                  &fstest.MapFile{Data: []byte(`{}`)},
			}
			result, err := projectmodel.DiscoverTSRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Roots).To(Equal([]string{"real"}))
		})
	})

	When("the snapshot filesystem itself cannot be read", func() {
		It("emits a ts_project_root_unavailable diagnostic instead of returning an error", func() {
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
		})
	})

	When("a budget is exceeded before discovery finishes walking", func() {
		It("truncates deterministically, marks Complete false, and emits ts_project_root_incomplete", func() {
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
		})
	})

	When("discovery never groups, orders, or labels its output by architectural role", func() {
		It("returns only bare, sorted directory paths for both Roots and Candidates", func() {
			snapshot := fstest.MapFS{
				"services/checkout/tsconfig.json": &fstest.MapFile{Data: []byte(`{}`)},
				"services/billing/tsconfig.json":  &fstest.MapFile{Data: []byte(`{}`)},
				"libs/shared/package.json":        &fstest.MapFile{Data: []byte(`{}`)},
			}
			result, err := projectmodel.DiscoverTSRoots(snapshot, projectmodel.GoBudgets{})
			Expect(err).NotTo(HaveOccurred())
			// Alphabetical path order only -- never reordered or clustered by
			// any inferred "service"/"lib" role, and TSRootDiscoveryResult has
			// no field capable of carrying such a label in the first place.
			Expect(result.Roots).To(Equal([]string{"services/billing", "services/checkout"}))
			Expect(result.Candidates).To(Equal([]string{"libs/shared"}))
		})
	})
})
