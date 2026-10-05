package githubingest_test

import (
	"context"
	"errors"

	"net/http"

	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// AC-5.7: a path resolving to a directory, symlink, or submodule surfaces as
// ErrUnsupportedContent.
func TestReadFile_UnsupportedContentTypeReturnsErrUnsupportedContent(t *testing.T) {
	tests := map[string]string{

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
			body_readerPart3Test_30(t, name, body)
		})
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
