package githubingest_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

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

// GitHub's Contents API returns base64 payloads split across lines with
// embedded newlines; Go's base64.StdEncoding.DecodeString already ignores
// \r and \n per its documented contract, so ReadFile must decode such
// payloads without alteration rather than treating them as malformed.
func TestReadFile_DecodesBase64ContentContainingEmbeddedNewlines(t *testing.T) {
	const canned = `{
		"type": "file",
		"encoding": "base64",
		"size": 11,
		"name": "split.txt",
		"path": "dir/split.txt",
		"sha": "splitsha",
		"content": "aGVs\nbG8g\nd29y\nbGQ="
	}`

	reader := newTestReader(t, func(req *http.Request) *http.Response {
		return jsonResponse(req, http.StatusOK, canned)
	})

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/split.txt"}
	content, _, err := reader.ReadFile(context.Background(), ref)
	if err != nil {
		t.Fatalf("ReadFile(%+v) for newline-split base64 content: got err %v, want nil", ref, err)
	}
	if string(content) != "hello world" {
		t.Fatalf("ReadFile(%+v) for newline-split base64 content: got %q, want %q", ref, content, "hello world")
	}
}
