package githubingest_test

import (
	"context"
	"errors"

	"net/http"

	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

func body_readerPart3Test_30(t *testing.T, name string, body string) {
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
