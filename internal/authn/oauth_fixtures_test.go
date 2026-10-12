package authn_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/coachapi"
)

const (
	oauthClientID     = "coach-oauth-client-id"
	oauthClientSecret = "coach-oauth-client-secret"
	oauthRedirectURI  = "http://coach.test/oauth/github/callback"
	oauthScenarioCode = "code-ok"
)

// errOAuthState fails Save and/or Consume so handlers can prove fail-closed 503.
type errOAuthState struct {
	saveErr    error
	consumeErr error
}

func (e *errOAuthState) Save(context.Context, string, time.Time) error {
	if e.saveErr != nil {
		return e.saveErr
	}
	return nil
}

func (e *errOAuthState) Consume(context.Context, string, time.Time) (bool, error) {
	if e.consumeErr != nil {
		return false, e.consumeErr
	}
	return true, nil
}

func newOAuthService(githubBase string, now func() time.Time, stateTTL time.Duration) *authn.Service {
	if now == nil {
		now = fixedNow(time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC))
	}
	if stateTTL <= 0 {
		stateTTL = 10 * time.Minute
	}
	svc, err := authn.New(authn.Options{
		SigningKey: []byte(testSecret),
		Issuer:     testIssuer,
		TokenTTL:   time.Hour,
		Now:        now,
		Denylist:   authn.NewMemoryDenylist(),
		GitHubOAuth: &authn.GitHubOAuthConfig{
			ClientID:     oauthClientID,
			ClientSecret: oauthClientSecret,
			BaseURL:      githubBase,
			RedirectURI:  oauthRedirectURI,
		},
		OAuthState:    authn.NewMemoryOAuthState(),
		OAuthStateTTL: stateTTL,
	})
	Expect(err).NotTo(HaveOccurred())
	return svc
}

func startOAuthAndParseState(client *http.Client, coachURL string) string {
	startResp, err := client.Get(coachURL + "/oauth/github/start")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { _ = startResp.Body.Close() })
	Expect(startResp.StatusCode).To(Equal(http.StatusFound))
	authURL, err := url.Parse(startResp.Header.Get("Location"))
	Expect(err).NotTo(HaveOccurred())
	state := authURL.Query().Get("state")
	Expect(state).NotTo(BeEmpty())
	return state
}

func expectInvalidRequest(code int, body []byte) {
	Expect(code).To(Equal(http.StatusBadRequest), "body=%s", body)
	env := decodeEnvelope(body)
	Expect(env.Error.Code).To(Equal(coachapi.ErrorCodeInvalidRequest))
}

func expectNoAccessToken(body []byte) {
	Expect(string(body)).NotTo(ContainSubstring("access_token"), "must not issue a token; body=%s", body)
}

// serveIncompleteUserIdentity is a one-host GitHub that exchanges any code
// for ghAccessToken and answers GET /user with user, which the specs make
// incomplete (zero id or empty login).
func serveIncompleteUserIdentity(user map[string]any, ghAccessToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/login/oauth/access_token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token": ghAccessToken,
				"token_type":   "bearer",
				"scope":        "",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			if r.Header.Get("Authorization") != "Bearer "+ghAccessToken {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(user)
		default:
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
		}
	}
}
