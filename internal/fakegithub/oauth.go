package fakegithub

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// oauthAuthorizeHandler answers GET /login/oauth/authorize. There is no
// browser consent step; the fake-only query param scenario_code selects which
// fixture OAuthCodeEntry key is issued. On a matching client_id it redirects
// to redirect_uri with that code and state.
func oauthAuthorizeHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		clientID := q.Get("client_id")
		redirectURI := q.Get("redirect_uri")
		state := q.Get("state")
		scenarioCode := q.Get("scenario_code")

		if clientID != fx.OAuth.ClientID {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
			http.Error(w, "fakegithub: unknown client_id", http.StatusUnauthorized)
			return
		}

		loc, err := url.Parse(redirectURI)
		if err != nil || redirectURI == "" || loc.Scheme == "" || loc.Host == "" {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
			http.Error(w, "fakegithub: invalid redirect_uri", http.StatusBadRequest)
			return
		}

		fx.mu.Lock()
		entry, ok := fx.OAuth.Codes[scenarioCode]
		fx.mu.Unlock()
		if !ok {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
			http.Error(w, "fakegithub: unknown scenario_code", http.StatusBadRequest)
			return
		}

		locQuery := loc.Query()
		locQuery.Set("code", scenarioCode)
		locQuery.Set("state", state)
		loc.RawQuery = locQuery.Encode()

		rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, string(entry.Scenario), r.Method, r.URL.Path, acceptanceharness.AuthModeNone))

		w.Header().Set("Location", loc.String())
		w.WriteHeader(http.StatusFound)
	}
}

// oauthTokenHandler answers POST /login/oauth/access_token. On ScenarioOK it
// mints a token into fx.OAuth.Tokens and deletes the code from Codes
// (single-use). Lookup and mutate run under fx.mu so concurrent exchanges of
// the same code cannot both succeed.
func oauthTokenHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeNone))
			http.Error(w, "fakegithub: invalid form body", http.StatusBadRequest)
			return
		}

		clientID := r.PostFormValue("client_id")
		clientSecret := r.PostFormValue("client_secret")
		code := r.PostFormValue("code")

		if clientID != fx.OAuth.ClientID || clientSecret != fx.OAuth.ClientSecret {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeNone))
			http.Error(w, "fakegithub: invalid client credentials", http.StatusUnauthorized)
			return
		}

		fx.mu.Lock()
		entry, ok := fx.OAuth.Codes[code]
		var token string
		if ok && entry.Scenario == ScenarioOK {
			token = newFakeToken()
			fx.OAuth.Tokens[token] = OAuthTokenEntry{IdentityLogin: entry.IdentityLogin, Scenario: ScenarioOK}
			delete(fx.OAuth.Codes, code)
		}
		fx.mu.Unlock()

		if !ok {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeNone))
			http.Error(w, "fakegithub: unknown code", http.StatusNotFound)
			return
		}

		rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, string(entry.Scenario), r.Method, r.URL.Path, acceptanceharness.AuthModeNone))

		switch entry.Scenario {
		case ScenarioNotFound:
			http.Error(w, "fakegithub: code not found", http.StatusNotFound)
			return
		case ScenarioAuthFail:
			http.Error(w, "fakegithub: auth failure", http.StatusUnauthorized)
			return
		case ScenarioTransient:
			http.Error(w, "fakegithub: transient upstream failure", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			Scope       string `json:"scope"`
		}{
			AccessToken: token,
			TokenType:   "bearer",
			Scope:       "",
		})
	}
}

// oauthUserHandler answers GET /user (and /api/v3/user). Installation,
// unknown, and rejected tokens are AuthModeRejected; a registered OAuth
// token dispatches on its Scenario.
func oauthUserHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)

		switch fx.ClassifyToken(token) {
		case TokenInstallation, TokenUnknown, TokenRejected:
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
			http.Error(w, "fakegithub: invalid or unknown token", http.StatusUnauthorized)
			return
		}

		fx.mu.Lock()
		entry := fx.OAuth.Tokens[token]
		fx.mu.Unlock()

		rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, string(entry.Scenario), r.Method, r.URL.Path, acceptanceharness.AuthModeOAuth))

		switch entry.Scenario {
		case ScenarioNotFound:
			http.Error(w, "fakegithub: identity not found", http.StatusNotFound)
			return
		case ScenarioAuthFail:
			http.Error(w, "fakegithub: auth failure", http.StatusUnauthorized)
			return
		case ScenarioTransient:
			http.Error(w, "fakegithub: transient upstream failure", http.StatusServiceUnavailable)
			return
		}

		identity, ok := fx.OAuth.Identities[entry.IdentityLogin]
		if !ok {
			http.Error(w, "fakegithub: token references an unregistered identity", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
		}{
			ID:    identity.ID,
			Login: identity.Login,
		})
	}
}

// extractBearerToken returns the credential from Authorization using the
// "token" or "Bearer" scheme, or "" if neither is present.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	for _, scheme := range []string{"token ", "Bearer "} {
		if strings.HasPrefix(auth, scheme) {
			return strings.TrimPrefix(auth, scheme)
		}
	}
	return ""
}

// newFakeToken returns a non-guessable access token from crypto/rand.
func newFakeToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// Fail loud rather than mint a predictable token.
		panic("fakegithub: crypto/rand failure: " + err.Error())
	}
	return "fake-oauth-" + hex.EncodeToString(buf)
}
