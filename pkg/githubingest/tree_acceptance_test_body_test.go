package githubingest_test

import (
	"context"
	"net/http"
	"sort"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

func body_treeAcceptanceTest_20(req *http.Request, byDir map[string]string) *http.Response {
	for dir, body := range byDir {
		var suffix string
		if dir == "" {
			suffix = "/contents/?"
		} else {
			suffix = "/contents/" + dir + "?"
		}
		if strings.Contains(req.URL.String(), suffix) {
			return jsonResponse(req, http.StatusOK, body)
		}
	}
	panic("treeContentsRouter: no fixture registered for request " + req.URL.String())
}

func body_treeAcceptanceTest_returnsExactlyTheMatchingFilePathsCorrectlyNeste_38() {
	reader := ginkgoTestReader(treeContentsRouter(map[string]string{
		"": `[
					{"type":"file","name":"main.go","path":"main.go","sha":"s1","size":10},
					{"type":"file","name":"README.md","path":"README.md","sha":"s2","size":5},
					{"type":"dir","name":"pkg","path":"pkg","sha":"s3","size":0}
				]`,
		"pkg": `[
					{"type":"file","name":"util.go","path":"pkg/util.go","sha":"s4","size":20},
					{"type":"dir","name":"sub","path":"pkg/sub","sha":"s5","size":0}
				]`,
		"pkg/sub": `[
					{"type":"file","name":"thing.ts","path":"pkg/sub/thing.ts","sha":"s6","size":30},
					{"type":"file","name":"notes.txt","path":"pkg/sub/notes.txt","sha":"s7","size":3}
				]`,
	}))

	ref := githubingest.GitHubTreeRef{Owner: "acme", Repo: "widgets", Ref: "main"}
	opts := githubingest.TreeListOptions{
		Filter: func(path string) bool {
			return strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".ts")
		},
	}

	result, err := reader.ListFiles(context.Background(), ref, opts)

	Expect(err).NotTo(HaveOccurred())
	paths := make([]string, 0, len(result))
	for _, e := range result {
		paths = append(paths, e.Path)
	}
	sort.Strings(paths)
	Expect(paths).To(Equal([]string{"main.go", "pkg/sub/thing.ts", "pkg/util.go"}))
}
