package fakegithub

import (
	"encoding/json"
	"net/http"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

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
		case ScenarioOK:
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
		case ScenarioNotFound:
			http.Error(w, "fakegithub: identity not found", http.StatusNotFound)
		case ScenarioAuthFail:
			http.Error(w, "fakegithub: auth failure", http.StatusUnauthorized)
		case ScenarioTransient:
			http.Error(w, "fakegithub: transient upstream failure", http.StatusServiceUnavailable)
		default:
			http.Error(w, "fakegithub: unknown scenario "+string(entry.Scenario), http.StatusInternalServerError)
		}
	}
}
