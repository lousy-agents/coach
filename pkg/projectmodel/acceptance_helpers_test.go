package projectmodel_test

import (
	"os"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func testMeta() projectmodel.SnapshotMeta {
	return projectmodel.SnapshotMeta{
		Revision:      "revision-sha",
		TreeID:        "tree-sha",
		ConfigDigest:  "config-digest",
		BackendDigest: "backend-digest",
	}
}

func file(content string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(content)}
}

func copyFixtureToTempDir(fixture string) string {
	dir := GinkgoT().TempDir()
	Expect(os.CopyFS(dir, os.DirFS(fixture))).To(Succeed())
	return dir
}

func edgeByTo(edges []projectmodel.ImportEdge, to string) (projectmodel.ImportEdge, bool) {
	for _, e := range edges {
		if e.To == to {
			return e, true
		}
	}
	return projectmodel.ImportEdge{}, false
}

func hasDiagnostic(diags []projectmodel.Diagnostic, code, path string) bool {
	for _, d := range diags {
		if d.Code == code && d.Path == path {
			return true
		}
	}
	return false
}

func diagnosticWithCode(diags []projectmodel.Diagnostic, code string) (projectmodel.Diagnostic, bool) {
	for _, d := range diags {
		if d.Code == code {
			return d, true
		}
	}
	return projectmodel.Diagnostic{}, false
}

func diagnosticsWithCode(diags []projectmodel.Diagnostic, code string) []projectmodel.Diagnostic {
	var out []projectmodel.Diagnostic
	for _, d := range diags {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}
