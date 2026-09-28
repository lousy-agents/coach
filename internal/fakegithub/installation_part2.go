package fakegithub

import (
	"net/http"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// rejectKnownNonAppBearer rejects OAuth, installation, and RejectedTokens
// bearers on App-JWT routes. The fake does not verify App JWT signatures
// (no key pair); TokenUnknown — including a missing header or unverifiable
// App JWT — is allowed through. That is an accepted simplification.
func rejectKnownNonAppBearer(fx *Fixture, rec *acceptanceharness.Recorder, w http.ResponseWriter, r *http.Request) bool {
	kind := fx.ClassifyToken(extractBearerToken(r))
	if kind == TokenOAuth || kind == TokenInstallation || kind == TokenRejected {
		rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
		writeJSONError(w, http.StatusUnauthorized, "Bad credentials")
		return true
	}
	return false
}

// writeScenarioStatus maps a non-OK Scenario to its HTTP status and body
// (GitHub-shaped JSON error). It reports whether scenario was handled; false
// means only ScenarioOK — the caller should write the success response.
// Empty, typo, and route-inappropriate values (e.g. ScenarioOversized on
// installation routes) fail loud with 500 so fixture mistakes cannot look
// like success.
func writeScenarioStatus(w http.ResponseWriter, scenario Scenario) bool {
	switch scenario {
	case ScenarioOK:
		return false
	case ScenarioNotFound:
		writeJSONError(w, http.StatusNotFound, "Not Found")
		return true
	case ScenarioAuthFail:
		writeJSONError(w, http.StatusUnauthorized, "Bad credentials")
		return true
	case ScenarioTransient:
		writeJSONError(w, http.StatusServiceUnavailable, "Service Unavailable")
		return true
	default:
		writeJSONError(w, http.StatusInternalServerError, "fakegithub: unknown scenario "+string(scenario))
		return true
	}
}
