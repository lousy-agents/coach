package githubingest_test

import (
	"context"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// treeContentsRouter dispatches a Contents API directory-listing request to
// the fixture registered for its directory path, so a test can describe a
// whole nested tree by directory rather than a single canned response.
func treeContentsRouter(byDir map[string]string) func(req *http.Request) *http.Response {
	return func(req *http.Request) *http.Response {
		return body_treeAcceptanceTest_20(req, byDir)
	}
}

var _ = Describe("repository tree listing (issue #101)", func() {
	Context("when a repository tree has a mix of matching and non-matching files across nested directories", func() {
		It("returns exactly the matching file paths, correctly nested", func() {
			body_treeAcceptanceTest_returnsExactlyTheMatchingFilePathsCorrectlyNeste_38()
		})
	})

	Context("when the repository tree is empty", func() {
		It("returns an empty result, not an error", func() {
			reader := ginkgoTestReader(treeContentsRouter(map[string]string{
				"": `[]`,
			}))

			ref := githubingest.GitHubTreeRef{Owner: "acme", Repo: "widgets", Ref: "main"}
			result, err := reader.ListFiles(context.Background(), ref, githubingest.TreeListOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
	})

	Context("when a directory listing fails with 401/403", func() {
		It("returns an error matching ErrAuth", func() {
			reader := ginkgoTestReader(func(req *http.Request) *http.Response {
				return jsonResponse(req, http.StatusForbidden, `{"message":"denied"}`)
			})

			ref := githubingest.GitHubTreeRef{Owner: "acme", Repo: "widgets", Ref: "main"}
			_, err := reader.ListFiles(context.Background(), ref, githubingest.TreeListOptions{})

			Expect(errors.Is(err, githubingest.ErrAuth)).To(BeTrue(), "got err %v, want errors.Is(err, ErrAuth)", err)
		})
	})

	Context("when the matching file set exceeds the configured budget", func() {
		It("returns an error matching ErrTooLarge for an over-count listing", func() {
			reader := ginkgoTestReader(treeContentsRouter(map[string]string{
				"": `[
					{"type":"file","name":"a.go","path":"a.go","sha":"s1","size":1},
					{"type":"file","name":"b.go","path":"b.go","sha":"s2","size":1},
					{"type":"file","name":"c.go","path":"c.go","sha":"s3","size":1}
				]`,
			}))

			ref := githubingest.GitHubTreeRef{Owner: "acme", Repo: "widgets", Ref: "main"}
			opts := githubingest.TreeListOptions{MaxFiles: 2}

			_, err := reader.ListFiles(context.Background(), ref, opts)

			Expect(errors.Is(err, githubingest.ErrTooLarge)).To(BeTrue(), "got err %v, want errors.Is(err, ErrTooLarge)", err)
		})

		It("returns an error matching ErrTooLarge for an over-byte-budget listing", func() {
			reader := ginkgoTestReader(treeContentsRouter(map[string]string{
				"": `[
					{"type":"file","name":"a.go","path":"a.go","sha":"s1","size":600},
					{"type":"file","name":"b.go","path":"b.go","sha":"s2","size":600}
				]`,
			}))

			ref := githubingest.GitHubTreeRef{Owner: "acme", Repo: "widgets", Ref: "main"}
			opts := githubingest.TreeListOptions{MaxTotalBytes: 1000}

			_, err := reader.ListFiles(context.Background(), ref, opts)

			Expect(errors.Is(err, githubingest.ErrTooLarge)).To(BeTrue(), "got err %v, want errors.Is(err, ErrTooLarge)", err)
		})
	})

	Context("when a directory listing contains a symlink or submodule entry", func() {
		It("skips it rather than walking into it or erroring", func() {
			reader := ginkgoTestReader(treeContentsRouter(map[string]string{
				"": `[
					{"type":"file","name":"main.go","path":"main.go","sha":"s1","size":10},
					{"type":"symlink","name":"escape","path":"escape","sha":"s2","size":9,"target":"../../etc"},
					{"type":"submodule","name":"vendor","path":"vendor","sha":"s3","size":0,"submodule_git_url":"git://example.com/lib.git"}
				]`,
			}))

			ref := githubingest.GitHubTreeRef{Owner: "acme", Repo: "widgets", Ref: "main"}
			result, err := reader.ListFiles(context.Background(), ref, githubingest.TreeListOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Path).To(Equal("main.go"))
		})
	})
})
