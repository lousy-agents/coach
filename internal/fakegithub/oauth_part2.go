package fakegithub

import (
	"crypto/rand"
	"encoding/hex"

	"net/http"

	"strings"
)

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

// exchangeOAuthCode looks up code and, on ScenarioOK, mints a single-use
// token into Tokens and deletes the code. Lookup and mutate run under
// fx.mu so concurrent exchanges of the same code cannot both succeed.
func (fx *Fixture) exchangeOAuthCode(code string) (token string, entry OAuthCodeEntry, ok bool) {
	fx.mu.Lock()
	defer fx.mu.Unlock()
	entry, ok = fx.OAuth.Codes[code]
	if !ok || entry.Scenario != ScenarioOK {
		return "", entry, ok
	}
	token = newFakeToken()
	fx.OAuth.Tokens[token] = OAuthTokenEntry{IdentityLogin: entry.IdentityLogin, Scenario: ScenarioOK}
	delete(fx.OAuth.Codes, code)
	return token, entry, true
}

// newFakeToken returns a non-guessable access token from crypto/rand.
func newFakeToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {

		panic("fakegithub: crypto/rand failure: " + err.Error())
	}
	return "fake-oauth-" + hex.EncodeToString(buf)
}
