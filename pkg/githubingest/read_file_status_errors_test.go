package githubingest_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

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
			expectStatusRejectedAsErrAuth(t, status)
		})
	}
}

func expectStatusRejectedAsErrAuth(t *testing.T, status int) {
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
			expectUnsupportedContentRejected(t, name, body)
		})
	}
}

func expectUnsupportedContentRejected(t *testing.T, name, body string) {
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
}
