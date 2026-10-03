package githubingest_test

import (
	"context"
	"errors"

	"net/http"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

func body_readerPart2Test_63(t *testing.T, cfg githubingest.GitHubAppConfig) {
	reader, err := githubingest.NewGitHubFileReader(cfg)
	if err == nil {
		t.Fatalf("NewGitHubFileReader(%+v): got nil error, want error for incomplete config", cfg)
	}
	if reader != nil {
		t.Fatalf("NewGitHubFileReader(%+v): got non-nil reader %v alongside error, want nil reader", cfg, reader)
	}
}

func body_readerPart2Test_84(t *testing.T, status int) {
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
