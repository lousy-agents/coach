package githubingest_test

import (
	"context"

	"net/http"

	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// urlRecordingEnterpriseTransport records every outbound request URL (in
// order) while answering both the ghinstallation token-mint request and the
// Contents API request, so a test can assert they share the same normalized
// Enterprise API base.
type urlRecordingEnterpriseTransport struct {
	seen []string
}

// tokenMintFailureTransport answers the ghinstallation token-mint request
// with the configured status and never expects to see a contents request,
// since a failed token mint should short-circuit ReadFile before go-github
// gets a chance to make the real Contents API call.
type tokenMintFailureTransport struct {
	status int
}

// AC-5.4: when ReadFile succeeds, it returns the decoded raw file bytes and
// metadata (path, ref, SHA, size), reading a fake GitHub Contents API
// response entirely offline.
func TestReadFile_ReturnsDecodedContentAndMetadataOnSuccess(t *testing.T) {
	const canned = `{
		"type": "file",
		"encoding": "base64",
		"size": 11,
		"name": "hello.txt",
		"path": "dir/hello.txt",
		"sha": "abc123sha",
		"content": "aGVsbG8gd29ybGQ="
	}`

	// The fake transport now sees two requests: the direct file fetch, and
	// the AC-5.7 parent-directory listing symlink check. Record every URL
	// seen and assert on the one that hit the exact file path, so the
	// directory-listing request's presence doesn't overwrite the assertion
	// target.
	var seenURLs []string
	reader := newTestReader(t, func(req *http.Request) *http.Response {
		seenURLs = append(seenURLs, req.URL.String())
		return jsonResponse(req, http.StatusOK, canned)
	})

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/hello.txt"}
	data, meta, err := reader.ReadFile(context.Background(), ref)
	if err != nil {
		t.Fatalf("ReadFile(%+v): unexpected error: %v", ref, err)
	}
	if string(data) != "hello world" {
		t.Fatalf("ReadFile(%+v) content: got %q, want %q", ref, data, "hello world")
	}
	wantMeta := githubingest.FileMetadata{Path: "dir/hello.txt", Ref: "main", SHA: "abc123sha", Size: 11}
	if meta != wantMeta {
		t.Fatalf("ReadFile(%+v) metadata: got %+v, want %+v", ref, meta, wantMeta)
	}

	// Regression guard raised by review: assert the outbound request's
	// owner/repo/path and ref query, not just the canned response body, so
	// a bug that requests the wrong endpoint (wrong owner, repo, path, or
	// ref) fails locally instead of silently passing against canned data.
	const wantContentsSuffix = "/repos/acme/widgets/contents/dir/hello.txt?ref=main"
	found := false
	for _, u := range seenURLs {
		if strings.HasSuffix(u, wantContentsSuffix) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ReadFile(%+v): outbound requests %v, want one ending with %q", ref, seenURLs, wantContentsSuffix)
	}
}

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

// AC-5.7 regression: GitHub's Contents API documents a special case where,
// if a symlink's target is a normal file within the same repository, the
// API transparently resolves it and returns the target file's content with
// type "file" -- not a symlink object. ReadFile must still reject it by
// listing the path's parent directory, which shows the raw (unresolved)
// entry for the symlink itself.
func TestReadFile_SymlinkTargetingInRepoFileReturnsErrUnsupportedContent(t *testing.T) {
	reader := givenReaderWhereContentsAPIResolvesASymlinkToItsTargetFile(t)
	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/link.txt"}

	_, _, err := reader.ReadFile(context.Background(), ref)

	thenErrorIs(t, err, githubingest.ErrUnsupportedContent, "ReadFile for a symlink whose target the Contents API resolved transparently")
}
