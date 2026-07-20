package fakegithub

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// installationTokenExpiresAt is the expiry timestamp the fake stamps onto
// every successfully minted installation token. It's a fixed, far-future
// RFC3339 timestamp rather than a computed one: the fake never actually
// expires tokens, so a stable value keeps responses deterministic across
// runs.
const installationTokenExpiresAt = "2999-01-01T00:00:00Z"

// writeScenarioStatus maps a non-OK Scenario to the HTTP status that models
// it, writes that status (with a JSON-decodable error body, mirroring
// real GitHub's error response shape), and reports whether scenario was one
// of the non-OK cases it handled. A false return means the caller should
// proceed to write its own ScenarioOK response.
func writeScenarioStatus(w http.ResponseWriter, scenario Scenario) bool {
	switch scenario {
	case ScenarioAuthFail:
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
		return true
	case ScenarioTransient:
		http.Error(w, `{"message":"Service Unavailable"}`, http.StatusServiceUnavailable)
		return true
	default:
		return false
	}
}

// installationTokenHandler answers
// POST /api/v3/app/installations/{id}/access_tokens. Deliberately does not
// verify the caller's "Authorization: Bearer <App JWT>" header: the fake
// doesn't hold the App's private/public key pair, so JWT verification is an
// accepted, documented simplification for this fake service.
func installationTokenHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, `{"message":"invalid installation id"}`, http.StatusBadRequest)
			return
		}

		entry, ok := fx.Installation.Installations[id]
		if !ok {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeNone))
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}

		rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, string(entry.Scenario), r.Method, r.URL.Path, acceptanceharness.AuthModeNone))

		if writeScenarioStatus(w, entry.Scenario) {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expires_at"`
		}{Token: entry.Token, ExpiresAt: installationTokenExpiresAt})
	}
}

// installationResolutionHandler answers
// GET /api/v3/repos/{owner}/{repo}/installation. Deliberately requires no
// credential: the fake models resolution as a lookup keyed purely on
// owner/repo, independent of any credential presented (or absent).
func installationResolutionHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("owner") + "/" + r.PathValue("repo")

		entry, ok := fx.Installation.RepoMappings[key]
		if !ok {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeNone))
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}

		rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, string(entry.Scenario), r.Method, r.URL.Path, acceptanceharness.AuthModeNone))

		if writeScenarioStatus(w, entry.Scenario) {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(struct {
			ID int64 `json:"id"`
		}{ID: entry.InstallationID})
	}
}

// permissionHandler answers
// GET /api/v3/repos/{owner}/{repo}/collaborators/{username}/permission. It
// requires an "Authorization: token <installation-token>" header classified
// by fx.ClassifyToken as a GitHub App installation token; any other token
// (including a misused OAuth access token) is rejected and recorded as
// acceptanceharness.AuthModeRejected rather than treated as a valid
// installation credential.
func permissionHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "token ")

		if fx.ClassifyToken(token) != TokenInstallation {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
			return
		}

		key := r.PathValue("owner") + "/" + r.PathValue("repo") + "/" + r.PathValue("username")

		entry, ok := fx.Installation.Permissions[key]
		if !ok {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeInstallation))
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
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
