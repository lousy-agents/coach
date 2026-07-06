package githubingest_test

import (
	"context"
	"errors"
	"net/http"
	"os"
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
//
// NOTE: at the time this test was written, pkg/semantics does not yet exist
// in this worktree/branch (Task 1's deliverable is still in-flight on a
// sibling branch and has not been merged here). The test skips gracefully
// in that case so it is not a false failure; once pkg/semantics is merged
// into this branch, the test becomes a real, meaningful guard.
func TestPackageBoundary_SemanticsDoesNotImportGithubDeps(t *testing.T) {
	if _, err := os.Stat("../semantics"); err != nil {
		t.Skip("pkg/semantics does not exist in this worktree yet; skipping regression guard until it is merged")
	}

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

	var gotContentsURL string
	reader := newTestReader(t, func(req *http.Request) *http.Response {
		gotContentsURL = req.URL.String()
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
	const wantContentsPath = "/repos/acme/widgets/contents/dir/hello.txt"
	if !strings.HasSuffix(gotContentsURL, wantContentsPath+"?ref=main") {
		t.Fatalf("ReadFile(%+v) request URL: got %q, want it to end with %q", ref, gotContentsURL, wantContentsPath+"?ref=main")
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
// did) sends the installation-token-mint request to the wrong path
// (".../app/installations/.../access_tokens" instead of
// ".../api/v3/app/installations/.../access_tokens"), so real Enterprise auth
// fails before ReadFile ever reaches the contents endpoint, even though the
// contents request URL alone looks correct.
func TestReadFile_EnterpriseBaseURLNormalizesTokenAndContentsRequestsToSameAPIBase(t *testing.T) {
	transport := &urlRecordingEnterpriseTransport{}

	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          1,
		InstallationID: 2,
		PrivateKey:     generateTestRSAPrivateKeyPEM(t),
		BaseURL:        "https://ghe.example.com/",
		Transport:      transport,
	})
	if err != nil {
		t.Fatalf("NewGitHubFileReader: unexpected error: %v", err)
	}

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "hello.txt"}
	if _, _, err := reader.ReadFile(context.Background(), ref); err != nil {
		t.Fatalf("ReadFile(%+v): unexpected error: %v", ref, err)
	}

	if len(transport.seen) != 2 {
		t.Fatalf("ReadFile(%+v): got %d outbound requests %v, want exactly 2 (token mint, then contents)", ref, len(transport.seen), transport.seen)
	}

	tokenURL, contentsURL := transport.seen[0], transport.seen[1]
	const wantAPIBase = "https://ghe.example.com/api/v3/"
	if !strings.HasPrefix(tokenURL, wantAPIBase) {
		t.Fatalf("token-mint request URL: got %q, want it to start with %q (the same normalized Enterprise API base go-github uses)", tokenURL, wantAPIBase)
	}
	if !strings.HasPrefix(contentsURL, wantAPIBase) {
		t.Fatalf("contents request URL: got %q, want it to start with %q", contentsURL, wantAPIBase)
	}

	const wantContentsSuffix = "/repos/acme/widgets/contents/hello.txt?ref=main"
	if !strings.HasSuffix(contentsURL, wantContentsSuffix) {
		t.Fatalf("contents request URL: got %q, want it to end with %q (owner/repo/path and ref query)", contentsURL, wantContentsSuffix)
	}
}
