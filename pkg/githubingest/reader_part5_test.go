package githubingest_test

import (
	"context"

	"fmt"
	"net/http"

	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

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

// whenReadFileSucceeds calls ReadFile and fails the test immediately if it
// returns an error, since the tests using it assert on requests recorded
// during a successful call, not on error-path behavior.
func whenReadFileSucceeds(t *testing.T, reader *githubingest.GitHubFileReader, ref githubingest.GitHubFileRef) {
	t.Helper()
	if _, _, err := reader.ReadFile(context.Background(), ref); err != nil {
		t.Fatalf("ReadFile(%+v): unexpected error: %v", ref, err)
	}
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
