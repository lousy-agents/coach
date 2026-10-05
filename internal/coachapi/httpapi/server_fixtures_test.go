package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

const (
	serverTestIssuer = "https://coach-api.test"
	serverTestSecret = "test-signing-secret-at-least-32-bytes!!"
)

// stubRepoAuthorizer records every Authorize call and returns a configured
// error (or nil) for every call.
type stubRepoAuthorizer struct {
	mu    sync.Mutex
	err   error
	calls []stubAuthorizeCall
}

type stubAuthorizeCall struct {
	login, owner, repo string
}

var _ authz.RepoAuthorizer = (*stubRepoAuthorizer)(nil)

func (s *stubRepoAuthorizer) Authorize(_ context.Context, login, owner, repo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, stubAuthorizeCall{login: login, owner: owner, repo: repo})
	return s.err
}

func (s *stubRepoAuthorizer) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// stubTaskQueue is a hand-rolled queue.TaskQueue test double; only Enqueue
// has real (configurable) behavior, matching the epic's guidance that Claim/
// Complete/Nack can be simple stubs for these HTTP-layer tests.
type stubTaskQueue struct {
	mu         sync.Mutex
	enqueueErr error
	enqueued   []queue.Task
}

var _ queue.TaskQueue = (*stubTaskQueue)(nil)

func (q *stubTaskQueue) Enqueue(_ context.Context, task queue.Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.enqueueErr != nil {
		return q.enqueueErr
	}
	q.enqueued = append(q.enqueued, task)
	return nil
}

func (q *stubTaskQueue) Claim(context.Context) (queue.Claim, bool, error) {
	return queue.Claim{}, false, nil
}

func (q *stubTaskQueue) Complete(context.Context, queue.Claim) error { return nil }

func (q *stubTaskQueue) Nack(context.Context, queue.Claim, bool) error { return nil }

func (q *stubTaskQueue) enqueuedTasks() []queue.Task {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]queue.Task, len(q.enqueued))
	copy(out, q.enqueued)
	return out
}

// serverErrDenylist always returns a store error from IsRevoked (fail-closed path).
type serverErrDenylist struct {
	err error
	mu  sync.Mutex
}

func (e *serverErrDenylist) IsRevoked(context.Context, string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return false, e.err
}

func (e *serverErrDenylist) Revoke(context.Context, string, time.Time) error {
	return nil
}

func newAuthnServiceForServer(opts authn.Options) *authn.Service {
	if opts.SigningKey == nil {
		opts.SigningKey = []byte(serverTestSecret)
	}
	if opts.Issuer == "" {
		opts.Issuer = serverTestIssuer
	}
	if opts.TokenTTL == 0 {
		opts.TokenTTL = time.Hour
	}
	if opts.Now == nil {
		opts.Now = serverFixedNow(time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC))
	}
	if opts.Denylist == nil {
		opts.Denylist = authn.NewMemoryDenylist()
	}
	svc, err := authn.New(opts)
	Expect(err).NotTo(HaveOccurred())
	return svc
}

func doServerReq(h http.Handler, method, path, bearer string, body []byte) (int, []byte) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func serverFixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func mustIssueToken(svc *authn.Service, p coachapi.Principal) string {
	tok, err := svc.Issue(context.Background(), p)
	Expect(err).NotTo(HaveOccurred())
	return tok
}

func decodeServerEnvelope(body []byte) coachapi.ErrorEnvelope {
	var env coachapi.ErrorEnvelope
	Expect(json.Unmarshal(body, &env)).To(Succeed(), "body=%s", body)
	return env
}

func expectEnvelope(code int, body []byte, wantStatus int, wantCode string) coachapi.ErrorEnvelope {
	Expect(code).To(Equal(wantStatus), "body=%s", body)
	env := decodeServerEnvelope(body)
	Expect(env.Error.Code).To(Equal(wantCode))
	Expect(strings.TrimSpace(env.Error.Message)).NotTo(BeEmpty())
	return env
}

func sequentialJobIDs(prefix string) func() string {
	n := 0
	return func() string {
		n++
		return fmt.Sprintf("%s-%d", prefix, n)
	}
}

func principalAlice() coachapi.Principal {
	return coachapi.Principal{Provider: "github", Subject: "1001", Login: "alice"}
}

func principalBob() coachapi.Principal {
	return coachapi.Principal{Provider: "github", Subject: "2002", Login: "bob"}
}

func validRepoBaselineScanBody() []byte {
	return []byte(`{"kind":"repo_baseline_scan","params":{"repo_owner":"acme","repo_name":"widgets"}}`)
}
