package fakegithub

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// installationTokenExpiresAt is a fixed far-future RFC3339 expiry stamped on
// minted installation tokens. The fake never expires tokens; a stable value
// keeps responses deterministic.
const installationTokenExpiresAt = "2999-01-01T00:00:00Z"

// writeScenarioStatus maps a non-OK Scenario to its HTTP status and body
// (GitHub-shaped JSON error). It reports whether scenario was handled; false
// means the caller should write the success response.
func writeScenarioStatus(w http.ResponseWriter, scenario Scenario) bool {
	switch scenario {
	case ScenarioNotFound:
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		return true
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

// rejectKnownNonAppBearer rejects OAuth, installation, and RejectedTokens
// bearers on App-JWT routes. The fake does not verify App JWT signatures
// (no key pair); TokenUnknown — including a missing header or unverifiable
// App JWT — is allowed through. That is an accepted simplification.
func rejectKnownNonAppBearer(fx *Fixture, rec *acceptanceharness.Recorder, w http.ResponseWriter, r *http.Request) bool {
	kind := fx.ClassifyToken(extractBearerToken(r))
	if kind == TokenOAuth || kind == TokenInstallation || kind == TokenRejected {
		rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
		return true
	}
	return false
}

// installationTokenHandler answers
// POST /api/v3/app/installations/{id}/access_tokens.
func installationTokenHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if rejectKnownNonAppBearer(fx, rec, w, r) {
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeNone))
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
// GET /api/v3/repos/{owner}/{repo}/installation. Resolution is keyed on
// owner/repo only after the App-bearer check in rejectKnownNonAppBearer.
func installationResolutionHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if rejectKnownNonAppBearer(fx, rec, w, r) {
			return
		}

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
// GET /api/v3/repos/{owner}/{repo}/collaborators/{username}/permission.
// Requires a classified installation token; any other credential is
// AuthModeRejected.
func permissionHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)

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
