package projectmodel_test

import (
	"os"
	"strings"
	"testing/fstest"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_goImportsAcceptanceTest_pinsTheModelCoverageCountsBudgetsVocabulary_99() {
	snapshot := os.DirFS("testdata/go_multiworkspace")
	model, err := projectmodel.BuildGoModel(snapshot, testMeta(), projectmodel.GoBuildOptions{})
	Expect(err).NotTo(HaveOccurred())

	Expect(model.Coverage.Complete).To(BeTrue())
	Expect(model.Coverage.Diagnostics).To(BeEmpty())
	Expect(model.Coverage.Counts).To(HaveKeyWithValue("unresolved_edges", 1))
	Expect(model.Coverage.Counts).To(HaveKeyWithValue("excluded_edges", 1))
	Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 2))
	Expect(model.Coverage.Counts).To(HaveKeyWithValue("packages_seen", 2))
	Expect(model.Coverage.Counts).To(HaveKeyWithValue("roots_seen", 3))
	for _, key := range []string{"wall_time_ms", "input_files", "input_bytes", "graph_nodes", "graph_edges", "working_set_bytes"} {
		Expect(model.Coverage.Budgets).To(HaveKey(key), "expected effective budget key %q", key)
	}
}

func body_goImportsAcceptanceTest_keepsTheFileSIdentityAndRecordsACoverageDiagnost_118() {
	// The invalid Go source below must stay an in-memory fstest.MapFS
	// entry rather than a real testdata/*.go file: a real one would
	// fail gofmt/go vet across the whole repo (see AGENTS.md's
	// mandatory "gofmt -l . must print nothing" check), since gofmt
	// walks every .go file on disk regardless of package boundaries.
	snapshot := fstest.MapFS{
		"go.mod":  &fstest.MapFile{Data: []byte("module example.com/syntaxerr\n\ngo 1.25\n")},
		"bad.go":  &fstest.MapFile{Data: []byte("package main\n\nfunc Broken( {\n")},
		"good.go": &fstest.MapFile{Data: []byte("package main\n\nimport \"fmt\"\n\nfunc Fine() {\n\tfmt.Println(\"fine\")\n}\n")},
	}
	model, err := projectmodel.BuildGoModel(snapshot, testMeta(), projectmodel.GoBuildOptions{})
	Expect(err).NotTo(HaveOccurred())

	Expect(model.Files).To(ContainElement(projectmodel.File{ID: "file:bad.go", Path: "bad.go", Language: "go"}))
	Expect(model.Files).To(ContainElement(projectmodel.File{ID: "file:good.go", Path: "good.go", Language: "go"}))

	Expect(hasDiagnostic(model.Coverage.Diagnostics, projectmodel.DiagFileSyntaxError, "bad.go")).To(BeTrue(),
		"expected a project_file_syntax_error diagnostic for bad.go, got %+v", model.Coverage.Diagnostics)

	for _, e := range model.ImportEdges {
		Expect(e.Site).NotTo(HavePrefix("bad.go:"), "bad.go must contribute no import edges, got %+v", e)
	}

	stdlibEdge, ok := edgeByTo(model.ImportEdges, "fmt")
	Expect(ok).To(BeTrue())
	Expect(stdlibEdge.Kind).To(Equal("stdlib"))
}

func body_goImportsAcceptanceTest_truncatesDeterministicallyMarksCompleteFalseAndR_149() {
	files := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/manyfiles\n\ngo 1.25\n")},
	}
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		files["pkg_"+name+".go"] = &fstest.MapFile{Data: []byte("package main\n\nfunc " + strings.ToUpper(name) + "() {}\n")}
	}

	unbounded, err := projectmodel.BuildGoModel(files, testMeta(), projectmodel.GoBuildOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(unbounded.Files).To(HaveLen(8))
	Expect(unbounded.Coverage.Complete).To(BeTrue())

	bounded, err := projectmodel.BuildGoModel(files, testMeta(), projectmodel.GoBuildOptions{
		Budgets: projectmodel.GoBudgets{MaxInputFiles: 1},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(bounded.Files).To(HaveLen(1), "expected the source-file phase itself to stop at the budget")
	Expect(bounded.Coverage.Complete).To(BeFalse())
	Expect(bounded.Coverage.Counts).To(HaveKeyWithValue("files_skipped", 7))

	analyzed := map[string]bool{}
	for _, f := range bounded.Files {
		analyzed[f.Path] = true
	}
	for _, m := range bounded.Modules {
		for _, p := range m.Files {
			Expect(analyzed).To(HaveKey(p), "Module.Files must not reference paths absent from Model.Files after truncation; orphan %q in %s", p, m.ID)
		}
	}
	for _, pkg := range bounded.Packages {
		for _, p := range pkg.Files {
			Expect(analyzed).To(HaveKey(p), "Package.Files must not reference paths absent from Model.Files after truncation; orphan %q in %s", p, pkg.ID)
		}
	}

	again, err := projectmodel.BuildGoModel(files, testMeta(), projectmodel.GoBuildOptions{
		Budgets: projectmodel.GoBudgets{MaxInputFiles: 1},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(again.Files).To(Equal(bounded.Files), "truncation must be deterministic across repeated calls")
}

func body_goImportsAcceptanceTest_alwaysResolvesTheImportToTheSameSortedFirstModul_288() {
	snapshot := os.DirFS("testdata/go_duplicate_module_path")

	var resolutions []string
	for i := 0; i < 30; i++ {
		model, err := projectmodel.BuildGoModel(snapshot, testMeta(), projectmodel.GoBuildOptions{})
		Expect(err).NotTo(HaveOccurred())
		edge, ok := edgeByTo(model.ImportEdges, "package:modA/pkg")
		Expect(ok).To(BeTrue(), "expected an edge resolving to package:modA/pkg, got %+v", model.ImportEdges)
		resolutions = append(resolutions, edge.To)
	}
	Expect(resolutions).To(HaveEach("package:modA/pkg"),
		"import resolution against colliding module paths must be deterministic, not map-iteration order")
}
