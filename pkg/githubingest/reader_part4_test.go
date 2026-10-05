package githubingest_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

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
				return body_readerPart4Test_97(req, resolvedFileContents, dirListingWithSymlinkEntry)
			},
		},
	})
	if err != nil {
		t.Fatalf("NewGitHubFileReader: unexpected error: %v", err)
	}
	return reader
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
			reader := newTestReader(t, (&sigTestReadFileDirectoryListingFailureMapsToSentinel43067476{fileContents: fileContents, tt: tt}).call)

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/hello.txt"}
			_, _, err := reader.ReadFile(context.Background(), ref)

			thenErrorIs(t, err, tt.want, fmt.Sprintf("ReadFile with the directory-listing symlink check failing %d", tt.status))
		})
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
