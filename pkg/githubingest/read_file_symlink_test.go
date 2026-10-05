package githubingest_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

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
			reader := newTestReader(t, serveFileThenFailDirectoryListing(fileContents, tt.status))

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/hello.txt"}
			_, _, err := reader.ReadFile(context.Background(), ref)

			thenErrorIs(t, err, tt.want, fmt.Sprintf("ReadFile with the directory-listing symlink check failing %d", tt.status))
		})
	}
}

// serveFileThenFailDirectoryListing answers the direct dir/hello.txt fetch
// with fileContents and fails every other Contents API call -- the AC-5.7
// parent-directory listing -- with status.
func serveFileThenFailDirectoryListing(fileContents string, status int) contentsHandlerFunc {
	return func(req *http.Request) *http.Response {
		if strings.HasSuffix(req.URL.Path, "/contents/dir/hello.txt") {
			return jsonResponse(req, http.StatusOK, fileContents)
		}
		return jsonResponse(req, status, `{"message":"denied"}`)
	}
}
