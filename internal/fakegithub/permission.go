package fakegithub

import (
	"encoding/json"
	"net/http"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// permissionHandler answers
// GET /api/v3/repos/{owner}/{repo}/collaborators/{username}/permission.
// Requires a classified installation token; any other credential is
// AuthModeRejected.
func permissionHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)

		if fx.ClassifyToken(token) != TokenInstallation {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
			writeJSONError(w, http.StatusUnauthorized, "Bad credentials")
			return
		}

		key := r.PathValue("owner") + "/" + r.PathValue("repo") + "/" + r.PathValue("username")

		entry, ok := fx.Installation.Permissions[key]
		if !ok {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeInstallation))
			writeJSONError(w, http.StatusNotFound, "Not Found")
			return
		}

		rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, string(entry.Scenario), r.Method, r.URL.Path, acceptanceharness.AuthModeInstallation))

		if writeScenarioStatus(w, entry.Scenario) {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(struct {
			Permission string `json:"permission"`
		}{Permission: entry.Level})
	}
}
