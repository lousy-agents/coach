package authn_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/coachapi"
)

const (
	testIssuer = "https://coach.test"
	testSecret = "test-signing-secret-at-least-32-bytes!!"
)

func fixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func newTestService(t *testing.T, opts authn.Options) *authn.Service {
	t.Helper()
	if opts.SigningKey == nil {
		opts.SigningKey = []byte(testSecret)
	}
	if opts.Issuer == "" {
		opts.Issuer = testIssuer
	}
	if opts.TokenTTL == 0 {
		opts.TokenTTL = time.Hour
	}
	if opts.Now == nil {
		opts.Now = fixedNow(time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC))
	}
	if opts.Denylist == nil {
		opts.Denylist = authn.NewMemoryDenylist()
	}
	svc, err := authn.New(opts)
	if err != nil {
		t.Fatalf("authn.New: %v", err)
	}
	return svc
}

func decodeEnvelope(t *testing.T, body []byte) coachapi.ErrorEnvelope {
	t.Helper()
	var env coachapi.ErrorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode error envelope: %v\nbody=%s", err, body)
	}
	return env
}

func doReq(t *testing.T, h http.Handler, method, path, bearer string, body []byte) (int, []byte) {
	t.Helper()
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

// Task 2a / Task A: missing, invalid, wrong-issuer, expired, and denylisted
// tokens are rejected on protected /v1 routes with 401 unauthenticated.
func TestProtectedRoute_RejectsBadTokensWith401(t *testing.T) {
	base := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	now := base
	// trackingDenylist records IsRevoked hits so the jti-denylisted case cannot
	// false-green on expiry alone (Validate checks exp before the denylist).
	mem := authn.NewMemoryDenylist()
	dl := &trackingDenylist{inner: mem}
	svc := newTestService(t, authn.Options{
		Now:      func() time.Time { return now },
		Denylist: dl,
	})
	h := svc.Handler()

	good, err := svc.Issue(context.Background(), coachapi.Principal{
		Provider: "github",
		Subject:  "12345",
		Login:    "octocat",
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Denylist a second token after issue (clock stays at base so the token is
	// still unexpired when Validate runs IsRevoked).
	toRevoke, err := svc.Issue(context.Background(), coachapi.Principal{
		Provider: "github",
		Subject:  "99999",
		Login:    "revoked-user",
	})
	if err != nil {
		t.Fatalf("Issue revoke candidate: %v", err)
	}
	if err := svc.Revoke(context.Background(), toRevoke); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// Wrong issuer token: mint with a different service issuer.
	other, err := authn.New(authn.Options{
		SigningKey: []byte(testSecret),
		Issuer:     "https://evil.example",
		TokenTTL:   time.Hour,
		Now:        func() time.Time { return base },
		Denylist:   authn.NewMemoryDenylist(),
	})
	if err != nil {
		t.Fatalf("other issuer New: %v", err)
	}
	wrongIss, err := other.Issue(context.Background(), coachapi.Principal{
		Provider: "github", Subject: "1", Login: "x",
	})
	if err != nil {
		t.Fatalf("wrong issuer Issue: %v", err)
	}

	// Expired: issue with short TTL; only that subtest advances the clock.
	short, err := authn.New(authn.Options{
		SigningKey: []byte(testSecret),
		Issuer:     testIssuer,
		TokenTTL:   time.Minute,
		Now:        func() time.Time { return base },
		Denylist:   authn.NewMemoryDenylist(),
	})
	if err != nil {
		t.Fatalf("short TTL New: %v", err)
	}
	expiredTok, err := short.Issue(context.Background(), coachapi.Principal{
		Provider: "github", Subject: "2", Login: "y",
	})
	if err != nil {
		t.Fatalf("expired Issue: %v", err)
	}

	cases := []struct {
		name        string
		bearer      string
		advance     time.Duration // per-case clock; zero keeps now at base
		wantRevoked bool          // true => must hit denylist IsRevoked and get revoked
	}{
		{name: "missing Authorization", bearer: ""},
		{name: "invalid signature / garbage", bearer: "not-a-jwt"},
		{name: "wrong issuer", bearer: wrongIss},
		{name: "expired", bearer: expiredTok, advance: 2 * time.Hour},
		{name: "jti denylisted", bearer: toRevoke, wantRevoked: true},
		{name: "github oauth access token stand-in", bearer: "gho_not_a_coach_jwt_at_all"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now = base.Add(tc.advance)
			t.Cleanup(func() { now = base })

			before := dl.isRevokedCalls()
			code, body := doReq(t, h, http.MethodGet, "/v1/me", tc.bearer, nil)
			if code != http.StatusUnauthorized {
				t.Fatalf("status: got %d want 401; body=%s", code, body)
			}
			env := decodeEnvelope(t, body)
			if env.Error.Code != coachapi.ErrorCodeUnauthenticated {
				t.Errorf("error.code: got %q want %q", env.Error.Code, coachapi.ErrorCodeUnauthenticated)
			}
			if strings.TrimSpace(env.Error.Message) == "" {
				t.Error("error.message must be non-empty")
			}
			if tc.wantRevoked {
				if dl.isRevokedCalls() <= before {
					t.Fatal("jti denylisted case must call IsRevoked (token still unexpired)")
				}
				if last, ok := dl.lastRevokedResult(); !ok || !last {
					t.Fatalf("IsRevoked must report revoked=true for denylisted jti; got ok=%v revoked=%v", ok, last)
				}
			}
		})
	}

	// Sanity: valid unexpired token authorizes /v1/me.
	now = base
	code, body := doReq(t, h, http.MethodGet, "/v1/me", good, nil)
	if code != http.StatusOK {
		t.Fatalf("valid token must authorize /v1/me: status=%d body=%s", code, body)
	}
}

// Task 2a / Task A: denylist store errors fail closed with 503 internal_error,
// distinct from denylisted-jti → 401.
func TestProtectedRoute_DenylistStoreError_503FailClosed(t *testing.T) {
	dl := &errDenylist{err: errors.New("denylist unavailable")}
	svc := newTestService(t, authn.Options{Denylist: dl})
	tok, err := svc.Issue(context.Background(), coachapi.Principal{
		Provider: "github", Subject: "1", Login: "octocat",
	})
	if err != nil {
		// Issue may not need denylist; if it fails, mint via a healthy service
		// with same key and validate against broken denylist service.
		healthy := newTestService(t, authn.Options{})
		tok, err = healthy.Issue(context.Background(), coachapi.Principal{
			Provider: "github", Subject: "1", Login: "octocat",
		})
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		svc = newTestService(t, authn.Options{Denylist: dl})
	}

	code, body := doReq(t, svc.Handler(), http.MethodGet, "/v1/me", tok, nil)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want 503; body=%s", code, body)
	}
	env := decodeEnvelope(t, body)
	if env.Error.Code != coachapi.ErrorCodeInternalError {
		t.Errorf("error.code: got %q want %q", env.Error.Code, coachapi.ErrorCodeInternalError)
	}
}

// Task 2a / Task A: test-mint is disabled by default (not registered → 404 not_found).
func TestTestMint_DisabledByDefault_Returns404(t *testing.T) {
	svc := newTestService(t, authn.Options{TestMintEnabled: false})
	body := []byte(`{"subject":"12345","login":"octocat"}`)
	code, resp := doReq(t, svc.Handler(), http.MethodPost, "/v1/auth/test-mint", "", body)
	if code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404; body=%s", code, resp)
	}
	env := decodeEnvelope(t, resp)
	if env.Error.Code != coachapi.ErrorCodeNotFound {
		t.Errorf("error.code: got %q want %q", env.Error.Code, coachapi.ErrorCodeNotFound)
	}
}

// Task 2a / Task A: when mint is enabled, mint succeeds and JWT authorizes /v1/me
// with Principal matching the mint request (provider=github).
func TestTestMint_Enabled_IssuesTokenThatAuthorizesMe(t *testing.T) {
	svc := newTestService(t, authn.Options{TestMintEnabled: true})
	h := svc.Handler()

	mintBody := []byte(`{"subject":"424242","login":"hubot"}`)
	code, resp := doReq(t, h, http.MethodPost, "/v1/auth/test-mint", "", mintBody)
	if code != http.StatusOK {
		t.Fatalf("mint status: got %d want 200; body=%s", code, resp)
	}
	var mintResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(resp, &mintResp); err != nil {
		t.Fatalf("mint response JSON: %v body=%s", err, resp)
	}
	if mintResp.Token == "" {
		t.Fatal("mint response token must be non-empty")
	}

	code, meBody := doReq(t, h, http.MethodGet, "/v1/me", mintResp.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("/v1/me status: got %d want 200; body=%s", code, meBody)
	}
	var p coachapi.Principal
	if err := json.Unmarshal(meBody, &p); err != nil {
		t.Fatalf("/v1/me JSON: %v body=%s", err, meBody)
	}
	want := coachapi.Principal{Provider: "github", Subject: "424242", Login: "hubot"}
	if p != want {
		t.Errorf("principal: got %+v want %+v", p, want)
	}
}

// Task 2a / Task A: issued Coach JWT carries provider, sub, login, iss, exp, jti
// and validates to the same Principal.
func TestIssueValidate_ClaimsAndPrincipal(t *testing.T) {
	svc := newTestService(t, authn.Options{})
	in := coachapi.Principal{Provider: "github", Subject: "12345", Login: "octocat"}
	tok, err := svc.Issue(context.Background(), in)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if tok == "" {
		t.Fatal("token must be non-empty")
	}
	// Three JWT segments.
	if parts := strings.Split(tok, "."); len(parts) != 3 {
		t.Fatalf("token must be compact JWT with 3 segments; got %d", len(parts))
	}

	got, err := svc.Validate(context.Background(), tok)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got != in {
		t.Errorf("Validate principal: got %+v want %+v", got, in)
	}

	claims, err := svc.InspectClaims(tok)
	if err != nil {
		t.Fatalf("InspectClaims: %v", err)
	}
	if claims.Provider != "github" {
		t.Errorf("provider: got %q", claims.Provider)
	}
	if claims.Subject != "12345" {
		t.Errorf("sub: got %q", claims.Subject)
	}
	if claims.Login != "octocat" {
		t.Errorf("login: got %q", claims.Login)
	}
	if claims.Issuer != testIssuer {
		t.Errorf("iss: got %q want %q", claims.Issuer, testIssuer)
	}
	if claims.ExpiresAt.IsZero() {
		t.Error("exp must be set")
	}
	if claims.ID == "" {
		t.Error("jti must be set")
	}
}

// Expiry is exclusive at exp: Validate must fail when Now equals ExpiresAt
// (not only when Now is strictly after exp).
func TestValidate_RejectsTokenAtExactExpiryBoundary(t *testing.T) {
	base := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	now := base
	const ttl = time.Hour
	svc := newTestService(t, authn.Options{
		TokenTTL: ttl,
		Now:      func() time.Time { return now },
	})
	tok, err := svc.Issue(context.Background(), coachapi.Principal{
		Provider: "github", Subject: "9", Login: "boundary",
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Still valid just before exp.
	now = base.Add(ttl - time.Nanosecond)
	if _, err := svc.Validate(context.Background(), tok); err != nil {
		t.Fatalf("Validate at exp-1ns: %v", err)
	}
	code, body := doReq(t, svc.Handler(), http.MethodGet, "/v1/me", tok, nil)
	if code != http.StatusOK {
		t.Fatalf("/v1/me at exp-1ns: got %d want 200 body=%s", code, body)
	}

	// At the exact expiry instant the token must be rejected.
	now = base.Add(ttl)
	if _, err := svc.Validate(context.Background(), tok); err == nil {
		t.Fatal("Validate at exp==now must fail")
	}
	code, body = doReq(t, svc.Handler(), http.MethodGet, "/v1/me", tok, nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("/v1/me at exp==now: got %d want 401 body=%s", code, body)
	}
	env := decodeEnvelope(t, body)
	if env.Error.Code != coachapi.ErrorCodeUnauthenticated {
		t.Errorf("error.code: got %q want %q", env.Error.Code, coachapi.ErrorCodeUnauthenticated)
	}
}

// Task 2a / Task A: Revoke denylists jti so subsequent Validate and /v1/me fail.
func TestRevoke_DenylistsJTI(t *testing.T) {
	svc := newTestService(t, authn.Options{})
	tok, err := svc.Issue(context.Background(), coachapi.Principal{
		Provider: "github", Subject: "7", Login: "revokee",
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if err := svc.Revoke(context.Background(), tok); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := svc.Validate(context.Background(), tok); err == nil {
		t.Fatal("Validate after Revoke must fail")
	}
	code, body := doReq(t, svc.Handler(), http.MethodGet, "/v1/me", tok, nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("status after revoke: got %d want 401 body=%s", code, body)
	}
	env := decodeEnvelope(t, body)
	if env.Error.Code != coachapi.ErrorCodeUnauthenticated {
		t.Errorf("error.code: got %q want %q", env.Error.Code, coachapi.ErrorCodeUnauthenticated)
	}
}

// errDenylist always returns a store error from IsRevoked (fail-closed path).
type errDenylist struct {
	err error
	mu  sync.Mutex
}

func (e *errDenylist) IsRevoked(context.Context, string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return false, e.err
}

func (e *errDenylist) Revoke(context.Context, string, time.Time) error {
	return nil
}

// trackingDenylist wraps a Denylist and records IsRevoked outcomes so tests can
// prove the denylist path ran (not merely expiry).
type trackingDenylist struct {
	inner authn.Denylist
	mu    sync.Mutex
	calls int
	last  *bool
}

func (t *trackingDenylist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	revoked, err := t.inner.IsRevoked(ctx, jti)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	v := revoked
	t.last = &v
	return revoked, err
}

func (t *trackingDenylist) Revoke(ctx context.Context, jti string, exp time.Time) error {
	return t.inner.Revoke(ctx, jti, exp)
}

func (t *trackingDenylist) isRevokedCalls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

func (t *trackingDenylist) lastRevokedResult() (revoked bool, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.last == nil {
		return false, false
	}
	return *t.last, true
}
