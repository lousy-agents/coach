package githubingest_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// AC-5.7 regression: the directory-listing symlink check must itself use
// GetContents's correct ref handling -- an empty Ref must default to the
// repository's default branch (a bare "/contents/{path}" URL, no ref query
// param at all) rather than producing the malformed URL the previously
// evaluated Git Trees API design would have built for an empty ref.
func TestReadFile_SucceedsWithEmptyRefDefaultingToDefaultBranch(t *testing.T) {
	const canned = `{
		"type": "file",
		"encoding": "base64",
		"size": 11,
		"name": "hello.txt",
		"path": "dir/hello.txt",
		"sha": "abc123sha",
		"content": "aGVsbG8gd29ybGQ="
	}`

	var seenURLs []string
	reader := newTestReader(t, func(req *http.Request) *http.Response {
		seenURLs = append(seenURLs, req.URL.String())
		return jsonResponse(req, http.StatusOK, canned)
	})

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "", Path: "dir/hello.txt"}
	_, _, err := reader.ReadFile(context.Background(), ref)
	thenErrorIs(t, err, nil, "ReadFile with an empty Ref (default branch)")

	for _, u := range seenURLs {
		if strings.Contains(u, "ref=") {
			t.Fatalf("ReadFile(%+v): outbound request %q carries a ref query param, want none for an empty Ref", ref, u)
		}
	}
	const wantDirListingSuffix = "/repos/acme/widgets/contents/dir"
	found := false
	for _, u := range seenURLs {
		if strings.HasSuffix(u, wantDirListingSuffix) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ReadFile(%+v): outbound requests %v, want one ending with %q (the AC-5.7 parent-directory listing)", ref, seenURLs, wantDirListingSuffix)
	}
}

// AC-5.7 regression: a slash-containing ref (e.g. a branch named
// "feature/x") must be percent-escaped into the ref query parameter for the
// directory-listing symlink check, exactly as GetContents already does for
// the direct file fetch -- not spliced unescaped into the URL path the way
// the previously evaluated Git Trees API design would have done.
func TestReadFile_SucceedsWithSlashContainingRef(t *testing.T) {
	const canned = `{
		"type": "file",
		"encoding": "base64",
		"size": 11,
		"name": "hello.txt",
		"path": "dir/hello.txt",
		"sha": "abc123sha",
		"content": "aGVsbG8gd29ybGQ="
	}`

	var seenURLs []string
	reader := newTestReader(t, func(req *http.Request) *http.Response {
		seenURLs = append(seenURLs, req.URL.String())
		return jsonResponse(req, http.StatusOK, canned)
	})

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "feature/x", Path: "dir/hello.txt"}
	_, _, err := reader.ReadFile(context.Background(), ref)
	thenErrorIs(t, err, nil, "ReadFile with a slash-containing ref")

	const wantDirListingSuffix = "/repos/acme/widgets/contents/dir?ref=feature%2Fx"
	found := false
	for _, u := range seenURLs {
		if strings.HasSuffix(u, wantDirListingSuffix) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ReadFile(%+v): outbound requests %v, want one ending with %q (ref percent-escaped, not spliced unescaped into the path)", ref, seenURLs, wantDirListingSuffix)
	}
}
