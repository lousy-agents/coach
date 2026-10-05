package fakegithub

import (
	"encoding/json"
	"net/http"
)

// writeJSONError writes a GitHub-shaped JSON error body with application/json.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Message string `json:"message"`
	}{Message: message})
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
