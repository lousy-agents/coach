package fakegithub

import (
	"encoding/json"
	"net/http"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// repoMetaHandler answers GET /api/v3/repos/{owner}/{repo} (default_branch).
func repoMetaHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if fx.ClassifyToken(token) != TokenInstallation {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
			writeJSONError(w, http.StatusUnauthorized, "Bad credentials")
			return
		}

		key := r.PathValue("owner") + "/" + r.PathValue("repo")
		entry, ok := fx.Repos.Repos[key]
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
			DefaultBranch string `json:"default_branch"`
			Name          string `json:"name"`
			FullName      string `json:"full_name"`
		}{
			DefaultBranch: entry.DefaultBranch,
			Name:          r.PathValue("repo"),
			FullName:      key,
		})
	}
}

// commitHandler answers GET /api/v3/repos/{owner}/{repo}/commits/{ref}.
func commitHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if fx.ClassifyToken(token) != TokenInstallation {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
			writeJSONError(w, http.StatusUnauthorized, "Bad credentials")
			return
		}

		key := r.PathValue("owner") + "/" + r.PathValue("repo") + "/" + r.PathValue("ref")
		entry, ok := fx.Repos.Commits[key]
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
			SHA string `json:"sha"`
		}{SHA: entry.SHA})
	}
}
