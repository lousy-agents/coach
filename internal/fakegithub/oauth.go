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

// oauthAuthorizeHandler answers GET /login/oauth/authorize. There's no real
// browser login/consent step to simulate, so the fake-only "scenario_code"
// query param lets a test pick which fixture-registered OAuthCodeEntry key
// gets issued directly: on a matching client_id, it redirects to
// redirect_uri with that code (echoed back verbatim) and the given state
// appended.
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
		if err != nil {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
			http.Error(w, "fakegithub: invalid redirect_uri", http.StatusBadRequest)
			return
		}

		locQuery := loc.Query()
		locQuery.Set("code", scenarioCode)
		locQuery.Set("state", state)
		loc.RawQuery = locQuery.Encode()

		entry := fx.OAuth.Codes[scenarioCode]
		rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, string(entry.Scenario), r.Method, r.URL.Path, acceptanceharness.AuthModeNone))

		w.Header().Set("Location", loc.String())
		w.WriteHeader(http.StatusFound)
	}
}

// oauthTokenHandler answers POST /login/oauth/access_token: it validates the
// form-encoded client_id/client_secret against the fixture, looks up code in
// fx.OAuth.Codes, and dispatches on the entry's Scenario. On ScenarioOK it
// mints a fresh access token, registers it into fx.OAuth.Tokens, and --
// mirroring real GitHub's single-use authorization codes -- deletes the
// consumed code from fx.OAuth.Codes.
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

		entry, ok := fx.OAuth.Codes[code]
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

		token := newFakeToken()
		fx.OAuth.Tokens[token] = OAuthTokenEntry{IdentityLogin: entry.IdentityLogin, Scenario: ScenarioOK}
		delete(fx.OAuth.Codes, code)

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

// oauthUserHandler answers GET /user: it extracts a bearer token from either
// the "token" or "Bearer" Authorization scheme, uses fx.ClassifyToken to
// reject an installation credential or an unknown token as misuse, and
// otherwise dispatches on the resolved OAuthTokenEntry's Scenario.
func oauthUserHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)

		switch fx.ClassifyToken(token) {
		case TokenInstallation, TokenUnknown:
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
			http.Error(w, "fakegithub: invalid or unknown token", http.StatusUnauthorized)
			return
		}

		entry := fx.OAuth.Tokens[token]

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

		identity := fx.OAuth.Identities[entry.IdentityLogin]

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

// extractBearerToken extracts the credential from an Authorization header
// using either the "token" (classic GitHub PAT/OAuth) or "Bearer" scheme,
// returning "" if neither is present.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	for _, scheme := range []string{"token ", "Bearer "} {
		if strings.HasPrefix(auth, scheme) {
			return strings.TrimPrefix(auth, scheme)
		}
	}
	return ""
}

// newFakeToken returns a fresh, non-guessable access token string, built
// entirely from crypto/rand output.
func newFakeToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand.Read failing is effectively unrecoverable; panicking
		// here (rather than silently minting a predictable token) keeps the
		// fake honest about the failure instead of masking it.
		panic("fakegithub: crypto/rand failure: " + err.Error())
	}
	return "fake-oauth-" + hex.EncodeToString(buf)
}
