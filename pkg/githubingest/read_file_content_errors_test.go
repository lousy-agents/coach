package githubingest_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

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
