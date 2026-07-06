package githubingest_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// AC-5.1: pkg/githubingest shall not import pkg/semantics.
func TestPackageBoundary_GithubingestDoesNotImportSemantics(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/lousy-agents/coach/pkg/githubingest/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps github.com/lousy-agents/coach/pkg/githubingest/... failed: %v\noutput:\n%s", err, out)
	}
	if strings.Contains(string(out), "lousy-agents/coach/pkg/semantics") {
		t.Fatalf("pkg/githubingest must not depend on pkg/semantics, but dependency list contained it:\n%s", out)
	}
}

// AC-1.1 regression guard: pkg/semantics shall not import the GitHub App
// dependencies introduced by this package (go-github, ghinstallation).
func TestPackageBoundary_SemanticsDoesNotImportGithubDeps(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/lousy-agents/coach/pkg/semantics/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps github.com/lousy-agents/coach/pkg/semantics/... failed: %v\noutput:\n%s", err, out)
	}
	if strings.Contains(string(out), "go-github") {
		t.Fatalf("pkg/semantics must not depend on go-github, but dependency list contained it:\n%s", out)
	}
	if strings.Contains(string(out), "ghinstallation") {
		t.Fatalf("pkg/semantics must not depend on ghinstallation, but dependency list contained it:\n%s", out)
	}
}

// AC-5.2: NewGitHubFileReader builds an authenticated client using
// ghinstallation/v2 wrapping the configured base http.RoundTripper, given a
// complete GitHubAppConfig. No network access occurs during construction.
func TestNewGitHubFileReader_BuildsAuthenticatedClientFromConfig(t *testing.T) {
	cfg := githubingest.GitHubAppConfig{
		AppID:          12345,
		InstallationID: 67890,
		PrivateKey:     generateTestRSAPrivateKeyPEM(t),
	}

	reader, err := githubingest.NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader with complete config: unexpected error: %v", err)
	}
	if reader == nil {
		t.Fatalf("NewGitHubFileReader with complete config: got nil reader, want non-nil")
	}
}

// AC-5.2: constructor validation - an incomplete GitHubAppConfig (missing
// AppID, InstallationID, or PrivateKey) must be rejected before any client
// is built.
func TestNewGitHubFileReader_RejectsIncompleteConfig(t *testing.T) {
	validKey := generateTestRSAPrivateKeyPEM(t)

	tests := map[string]githubingest.GitHubAppConfig{
		"missing AppID": {
			InstallationID: 67890,
			PrivateKey:     validKey,
		},
		"missing InstallationID": {
			AppID:      12345,
			PrivateKey: validKey,
		},
		"missing PrivateKey": {
			AppID:          12345,
			InstallationID: 67890,
		},
		// Regression guard raised by review: negative IDs are not valid
		// GitHub App identifiers and must be rejected alongside zero/missing
		// ones, rather than producing a reader with impossible configuration
		// that only fails later during auth.
		"negative AppID": {
			AppID:          -1,
			InstallationID: 67890,
			PrivateKey:     validKey,
		},
		"negative InstallationID": {
			AppID:          12345,
			InstallationID: -1,
			PrivateKey:     validKey,
		},
	}

	for name, cfg := range tests {
		t.Run(name, func(t *testing.T) {
			reader, err := githubingest.NewGitHubFileReader(cfg)
			if err == nil {
				t.Fatalf("NewGitHubFileReader(%+v): got nil error, want error for incomplete config", cfg)
			}
			if reader != nil {
				t.Fatalf("NewGitHubFileReader(%+v): got non-nil reader %v alongside error, want nil reader", cfg, reader)
			}
		})
	}
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

// AC-5.5: a 404 response from the Contents API surfaces as ErrNotFound.
func TestReadFile_NotFoundStatusReturnsErrNotFound(t *testing.T) {
	reader := newTestReader(t, func(req *http.Request) *http.Response {
		return jsonResponse(req, http.StatusNotFound, `{"message":"Not Found"}`)
	})

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "missing.txt"}
	_, _, err := reader.ReadFile(context.Background(), ref)
	if err == nil {
		t.Fatalf("ReadFile(%+v): got nil error, want an error wrapping ErrNotFound for a 404 response", ref)
	}
	if !errors.Is(err, githubingest.ErrNotFound) {
		t.Fatalf("ReadFile(%+v) on 404: got err %v, want errors.Is(err, ErrNotFound) to hold", ref, err)
	}
}

// AC-5.6: 401 and 403 responses from the Contents API both surface as
// ErrAuth.
func TestReadFile_UnauthorizedOrForbiddenStatusReturnsErrAuth(t *testing.T) {
	tests := map[string]int{
		"401 Unauthorized": http.StatusUnauthorized,
		"403 Forbidden":    http.StatusForbidden,
	}

	for name, status := range tests {
		t.Run(name, func(t *testing.T) {
			reader := newTestReader(t, func(req *http.Request) *http.Response {
				return jsonResponse(req, status, `{"message":"denied"}`)
			})

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "secret.txt"}
			_, _, err := reader.ReadFile(context.Background(), ref)
			if err == nil {
				t.Fatalf("ReadFile(%+v): got nil error, want an error wrapping ErrAuth for a %d response", ref, status)
			}
			if !errors.Is(err, githubingest.ErrAuth) {
				t.Fatalf("ReadFile(%+v) on %d: got err %v, want errors.Is(err, ErrAuth) to hold", ref, status, err)
			}
		})
	}
}

// AC-5.7: a path resolving to a directory, symlink, or submodule surfaces as
// ErrUnsupportedContent.
func TestReadFile_UnsupportedContentTypeReturnsErrUnsupportedContent(t *testing.T) {
	tests := map[string]string{
		// A directory listing comes back as a JSON array rather than a
		// single file object.
		"directory listing (JSON array)": `[
			{"type":"file","name":"a.txt","path":"dir/a.txt","sha":"a","size":1},
			{"type":"file","name":"b.txt","path":"dir/b.txt","sha":"b","size":1}
		]`,
		"symlink": `{"type":"symlink","name":"link","path":"dir/link","sha":"sha1","size":9,"target":"../elsewhere"}`,
		"submodule": `{"type":"submodule","name":"vendor/lib","path":"vendor/lib","sha":"sha2","size":0,` +
			`"submodule_git_url":"git://example.com/lib.git"}`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			reader := newTestReader(t, func(req *http.Request) *http.Response {
				return jsonResponse(req, http.StatusOK, body)
			})

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir"}
			_, _, err := reader.ReadFile(context.Background(), ref)
			if err == nil {
				t.Fatalf("ReadFile(%+v) for %s: got nil error, want an error wrapping ErrUnsupportedContent", ref, name)
			}
			if !errors.Is(err, githubingest.ErrUnsupportedContent) {
				t.Fatalf("ReadFile(%+v) for %s: got err %v, want errors.Is(err, ErrUnsupportedContent) to hold", ref, name, err)
			}
		})
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

// givenReaderWhereContentsAPIResolvesASymlinkToItsTargetFile builds a reader
// whose fake Contents API response for the exact file path "dir/link.txt"
// reports type "file" (as GitHub does for an in-repo symlink target), while
// its response for the parent directory listing ("dir") reports that same
// name's raw entry as type "symlink" -- the case only the directory-listing
// check, not the direct file request's type field, can catch.
func givenReaderWhereContentsAPIResolvesASymlinkToItsTargetFile(t *testing.T) *githubingest.GitHubFileReader {
	t.Helper()

	const resolvedFileContents = `{
		"type": "file",
		"encoding": "base64",
		"size": 11,
		"name": "link.txt",
		"path": "dir/link.txt",
		"sha": "targetsha",
		"content": "aGVsbG8gd29ybGQ="
	}`
	const dirListingWithSymlinkEntry = `[
		{"type": "symlink", "name": "link.txt", "path": "dir/link.txt", "sha": "linksha", "size": 9, "target": "../elsewhere"},
		{"type": "file", "name": "hello.txt", "path": "dir/hello.txt", "sha": "hellosha", "size": 5}
	]`

	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          1,
		InstallationID: 2,
		PrivateKey:     generateTestRSAPrivateKeyPEM(t),
		Transport: &fakeGitHubTransport{
			handleContents: func(req *http.Request) *http.Response {
				if strings.HasSuffix(req.URL.Path, "/contents/dir/link.txt") {
					return jsonResponse(req, http.StatusOK, resolvedFileContents)
				}
				return jsonResponse(req, http.StatusOK, dirListingWithSymlinkEntry)
			},
		},
	})
	if err != nil {
		t.Fatalf("NewGitHubFileReader: unexpected error: %v", err)
	}
	return reader
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

// AC-5.5/AC-5.6 regression: a 401/403/404 response from the AC-5.7
// parent-directory listing request -- as opposed to the direct file fetch --
// must map through the same mapContentsAPIError sentinels as any other
// Contents API call.
func TestReadFile_DirectoryListingFailureMapsToSentinel(t *testing.T) {
	const fileContents = `{
		"type": "file",
		"encoding": "base64",
		"size": 11,
		"name": "hello.txt",
		"path": "dir/hello.txt",
		"sha": "abc123sha",
		"content": "aGVsbG8gd29ybGQ="
	}`

	tests := []struct {
		name   string
		status int
		want   error
	}{
		{"401 on directory listing", http.StatusUnauthorized, githubingest.ErrAuth},
		{"403 on directory listing", http.StatusForbidden, githubingest.ErrAuth},
		{"404 on directory listing", http.StatusNotFound, githubingest.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := newTestReader(t, func(req *http.Request) *http.Response {
				if strings.HasSuffix(req.URL.Path, "/contents/dir/hello.txt") {
					return jsonResponse(req, http.StatusOK, fileContents)
				}
				return jsonResponse(req, tt.status, `{"message":"denied"}`)
			})

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/hello.txt"}
			_, _, err := reader.ReadFile(context.Background(), ref)

			thenErrorIs(t, err, tt.want, fmt.Sprintf("ReadFile with the directory-listing symlink check failing %d", tt.status))
		})
	}
}

// AC-5.8: a reported size over the Contents API's 1 MiB limit surfaces as
// ErrTooLarge, checked before any attempt to decode (possibly truncated)
// content, and no bytes are returned.
func TestReadFile_OversizedFileReturnsErrTooLarge(t *testing.T) {
	// encoding "none" and garbage content mimic what the real API sends for
	// files over the limit: content is not usable, so a correct
	// implementation must reject based on size before ever touching it.
	const canned = `{
		"type": "file",
		"encoding": "none",
		"size": 1048577,
		"name": "big.bin",
		"path": "dir/big.bin",
		"sha": "bigsha",
		"content": "not-valid-base64-and-must-never-be-decoded"
	}`

	reader := newTestReader(t, func(req *http.Request) *http.Response {
		return jsonResponse(req, http.StatusOK, canned)
	})

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/big.bin"}
	data, _, err := reader.ReadFile(context.Background(), ref)
	if err == nil {
		t.Fatalf("ReadFile(%+v): got nil error, want an error wrapping ErrTooLarge for a 1048577 byte file", ref)
	}
	if !errors.Is(err, githubingest.ErrTooLarge) {
		t.Fatalf("ReadFile(%+v) for oversized file: got err %v, want errors.Is(err, ErrTooLarge) to hold", ref, err)
	}
	if data != nil {
		t.Fatalf("ReadFile(%+v) for oversized file: got non-nil bytes %q alongside error, want no bytes returned", ref, data)
	}
}

// AC-5.9: content that decodes to zero bytes surfaces as ErrEmptyContent.
func TestReadFile_EmptyContentReturnsErrEmptyContent(t *testing.T) {
	const canned = `{
		"type": "file",
		"encoding": "base64",
		"size": 0,
		"name": "empty.txt",
		"path": "dir/empty.txt",
		"sha": "emptysha",
		"content": ""
	}`

	reader := newTestReader(t, func(req *http.Request) *http.Response {
		return jsonResponse(req, http.StatusOK, canned)
	})

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/empty.txt"}
	_, _, err := reader.ReadFile(context.Background(), ref)
	if err == nil {
		t.Fatalf("ReadFile(%+v): got nil error, want an error wrapping ErrEmptyContent for empty content", ref)
	}
	if !errors.Is(err, githubingest.ErrEmptyContent) {
		t.Fatalf("ReadFile(%+v) for empty content: got err %v, want errors.Is(err, ErrEmptyContent) to hold", ref, err)
	}
}

// AC-5.11: when the returned content fails to base64-decode, ReadFile
// returns a non-nil, wrapped API-failure error that does not match any of
// the five defined sentinels.
func TestReadFile_UndecodableContentReturnsErrorNotMatchingAnySentinel(t *testing.T) {
	const canned = `{
		"type": "file",
		"encoding": "base64",
		"size": 5,
		"name": "bad.txt",
		"path": "dir/bad.txt",
		"sha": "badsha",
		"content": "!!!not-valid-base64!!!"
	}`

	reader := newTestReader(t, func(req *http.Request) *http.Response {
		return jsonResponse(req, http.StatusOK, canned)
	})

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/bad.txt"}
	_, _, err := reader.ReadFile(context.Background(), ref)
	if err == nil {
		t.Fatalf("ReadFile(%+v): got nil error, want a wrapped error for undecodable base64 content", ref)
	}

	sentinels := []error{
		githubingest.ErrAuth,
		githubingest.ErrNotFound,
		githubingest.ErrUnsupportedContent,
		githubingest.ErrEmptyContent,
		githubingest.ErrTooLarge,
	}
	for _, sentinel := range sentinels {
		if errors.Is(err, sentinel) {
			t.Fatalf("ReadFile(%+v) for undecodable content: got err %v, want it NOT to match sentinel %v", ref, err, sentinel)
		}
	}
}

// Regression guard raised by review: a response reporting encoding "base64"
// but omitting the content field entirely (Content == nil) must return a
// wrapped, non-panicking error rather than dereferencing a nil pointer.
func TestReadFile_NilContentReturnsErrorWithoutPanicking(t *testing.T) {
	const canned = `{
		"type": "file",
		"encoding": "base64",
		"size": 0,
		"name": "nil.txt",
		"path": "dir/nil.txt",
		"sha": "nilsha"
	}`

	reader := newTestReader(t, func(req *http.Request) *http.Response {
		return jsonResponse(req, http.StatusOK, canned)
	})

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/nil.txt"}
	_, _, err := reader.ReadFile(context.Background(), ref)
	if err == nil {
		t.Fatalf("ReadFile(%+v): got nil error, want a wrapped error for a response with no content field", ref)
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

// urlRecordingEnterpriseTransport records every outbound request URL (in
// order) while answering both the ghinstallation token-mint request and the
// Contents API request, so a test can assert they share the same normalized
// Enterprise API base.
type urlRecordingEnterpriseTransport struct {
	seen []string
}

func (u *urlRecordingEnterpriseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u.seen = append(u.seen, req.URL.String())
	if strings.HasSuffix(req.URL.Path, "/access_tokens") {
		return mintInstallationTokenResponse(req), nil
	}
	// Both the direct file request and the AC-5.7 parent-directory listing
	// request get this same canned single-file response; parsed as a
	// directory listing it has no entries, so the symlink check finds
	// nothing and ReadFile proceeds -- exactly the "not a symlink" default
	// this test needs.
	const canned = `{
		"type": "file",
		"encoding": "base64",
		"size": 5,
		"name": "hello.txt",
		"path": "hello.txt",
		"sha": "deadbeef",
		"content": "aGVsbG8="
	}`
	return jsonResponse(req, http.StatusOK, canned), nil
}

// AC-5.3 regression: go-github's WithEnterpriseURLs and ghinstallation's
// Transport.BaseURL normalize a bare Enterprise host differently -- go-github
// appends "api/v3/" to the path, ghinstallation does not. Passing the raw
// caller-supplied BaseURL straight to ghinstallation (as the code previously
// did) sent the installation-token-mint request to the wrong path
// (".../app/installations/.../access_tokens" instead of
// ".../api/v3/app/installations/.../access_tokens"), so real Enterprise auth
// would fail before ReadFile ever reached the contents endpoint, even though
// the contents request URL alone looked correct.
func TestReadFile_EnterpriseBaseURLNormalizesTokenAndContentsRequestsToSameAPIBase(t *testing.T) {
	reader, transport := givenEnterpriseReader(t, "https://ghe.example.com/")
	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "hello.txt"}

	whenReadFileSucceeds(t, reader, ref)

	if len(transport.seen) != 3 {
		t.Fatalf("ReadFile(%+v): got %d outbound requests %v, want exactly 3 (token mint, contents, then the AC-5.7 directory-listing symlink check)", ref, len(transport.seen), transport.seen)
	}
	tokenURL, contentsURL, dirListingURL := transport.seen[0], transport.seen[1], transport.seen[2]

	const wantAPIBase = "https://ghe.example.com/api/v3/"
	thenRequestTargets(t, "token-mint", tokenURL, wantAPIBase, "")
	thenRequestTargets(t, "contents", contentsURL, wantAPIBase, "/repos/acme/widgets/contents/hello.txt?ref=main")
	thenRequestTargets(t, "directory listing", dirListingURL, wantAPIBase, "?ref=main")
}

// givenEnterpriseReader builds a reader configured with baseURL as its
// GitHub Enterprise BaseURL, wired to a transport that records every
// outbound request URL for inspection.
func givenEnterpriseReader(t *testing.T, baseURL string) (*githubingest.GitHubFileReader, *urlRecordingEnterpriseTransport) {
	t.Helper()

	transport := &urlRecordingEnterpriseTransport{}
	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          1,
		InstallationID: 2,
		PrivateKey:     generateTestRSAPrivateKeyPEM(t),
		BaseURL:        baseURL,
		Transport:      transport,
	})
	if err != nil {
		t.Fatalf("NewGitHubFileReader: unexpected error: %v", err)
	}
	return reader, transport
}

// whenReadFileSucceeds calls ReadFile and fails the test immediately if it
// returns an error, since the tests using it assert on requests recorded
// during a successful call, not on error-path behavior.
func whenReadFileSucceeds(t *testing.T, reader *githubingest.GitHubFileReader, ref githubingest.GitHubFileRef) {
	t.Helper()
	if _, _, err := reader.ReadFile(context.Background(), ref); err != nil {
		t.Fatalf("ReadFile(%+v): unexpected error: %v", ref, err)
	}
}

// thenRequestTargets fails the test unless url starts with wantAPIBase and
// (when wantSuffix is non-empty) ends with wantSuffix.
func thenRequestTargets(t *testing.T, name, url, wantAPIBase, wantSuffix string) {
	t.Helper()
	if !strings.HasPrefix(url, wantAPIBase) {
		t.Fatalf("%s request URL: got %q, want it to start with %q (the same normalized Enterprise API base go-github uses)", name, url, wantAPIBase)
	}
	if wantSuffix != "" && !strings.HasSuffix(url, wantSuffix) {
		t.Fatalf("%s request URL: got %q, want it to end with %q", name, url, wantSuffix)
	}
}

// tokenMintFailureTransport answers the ghinstallation token-mint request
// with the configured status and never expects to see a contents request,
// since a failed token mint should short-circuit ReadFile before go-github
// gets a chance to make the real Contents API call.
type tokenMintFailureTransport struct {
	status int
}

func (f *tokenMintFailureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.HasSuffix(req.URL.Path, "/access_tokens") {
		panic(fmt.Sprintf("unexpected contents request %s after a failed token mint", req.URL))
	}
	return jsonResponse(req, f.status, `{"message":"Bad credentials"}`), nil
}

// AC-5.6 regression: when the ghinstallation token-mint request itself fails
// with 401/403 (e.g. a revoked or misconfigured GitHub App installation),
// go-github never issues the actual Contents API request, so ReadFile's
// error has no *http.Response of its own to inspect -- the failure is
// wrapped inside a *ghinstallation.HTTPError in the error chain instead.
// Previously this surfaced as a generic wrapped error rather than ErrAuth.
func TestReadFile_TokenMintAuthFailureReturnsErrAuth(t *testing.T) {
	statuses := map[string]int{
		"401 Unauthorized": http.StatusUnauthorized,
		"403 Forbidden":    http.StatusForbidden,
		// Regression guard raised by review: a 404 from the token-mint
		// endpoint (e.g. an unknown or revoked InstallationID) previously
		// surfaced as ErrNotFound ("file not found"), misreporting a broken
		// GitHub App installation as a missing file. AC-5.5's
		// 404 -> ErrNotFound scope is the Contents API's own response, not
		// the token-mint endpoint's -- any token-mint failure is an
		// authentication/configuration problem, so it must be ErrAuth
		// regardless of the specific status code involved.
		"404 Not Found (wrong InstallationID)": http.StatusNotFound,
	}

	for name, status := range statuses {
		t.Run(name, func(t *testing.T) {
			reader := givenReaderWithTokenMintFailing(t, status)
			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "secret.txt"}

			_, _, err := reader.ReadFile(context.Background(), ref)

			thenErrorIs(t, err, githubingest.ErrAuth, fmt.Sprintf("ReadFile with token mint failing %d", status))
		})
	}
}

// givenReaderWithTokenMintFailing builds a reader whose transport fails the
// ghinstallation token-mint request itself with status, before go-github
// ever gets a chance to make the real Contents API call -- the transport
// panics if it sees a contents request, since one reaching it would mean
// ReadFile failed to short-circuit on the token-mint failure.
func givenReaderWithTokenMintFailing(t *testing.T, status int) *githubingest.GitHubFileReader {
	t.Helper()

	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          1,
		InstallationID: 2,
		PrivateKey:     generateTestRSAPrivateKeyPEM(t),
		Transport:      &tokenMintFailureTransport{status: status},
	})
	if err != nil {
		t.Fatalf("NewGitHubFileReader: unexpected error: %v", err)
	}
	return reader
}
