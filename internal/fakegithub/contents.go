package fakegithub

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"path"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// contentsHandler answers GET /api/v3/repos/{owner}/{repo}/contents/{path...}.
// githubingest ReadFile hits this route twice: file content, then (after type
// "file") parent-dir listing for symlink detection. Dispatch is fixture-only:
// Files wins, else Dirs, else 404. Requires an installation token; other
// credentials are AuthModeRejected. Scenario failures still record
// AuthModeInstallation (credential was valid; resource outcome differs).
func contentsHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if fx.ClassifyToken(token) != TokenInstallation {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeRejected))
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
			return
		}

		reqPath := r.PathValue("path")
		key := contentsKey(r.PathValue("owner"), r.PathValue("repo"), r.URL.Query().Get("ref"), reqPath)

		if entry, ok := fx.Contents.Files[key]; ok {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, string(entry.Scenario), r.Method, r.URL.Path, acceptanceharness.AuthModeInstallation))
			if writeScenarioStatus(w, entry.Scenario) {
				return
			}
			writeContentsFileResponse(w, reqPath, entry)
			return
		}

		if dirEntries, ok := fx.Contents.Dirs[key]; ok {
			rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeInstallation))
			writeContentsDirResponse(w, reqPath, dirEntries)
			return
		}

		rec.Record(acceptanceharness.NewRequestRecord(fx.Header.FixtureID, "", r.Method, r.URL.Path, acceptanceharness.AuthModeInstallation))
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}
}

// contentsKey builds the "owner/repo/ref/path" lookup key shared by Files and
// Dirs. path.Join drops empty segments, so a root path collapses to
// "owner/repo/ref".
func contentsKey(owner, repo, ref, p string) string {
	return path.Join(owner, repo, ref, p)
}

// writeContentsFileResponse writes a github.RepositoryContent-shaped file
// object. ScenarioOversized uses encoding "none" and empty content (real
// GitHub oversize shape); githubingest checks size before decoding content.
func writeContentsFileResponse(w http.ResponseWriter, reqPath string, entry FileEntry) {
	encoding := "base64"
	content := base64.StdEncoding.EncodeToString(entry.Content)
	if entry.Scenario == ScenarioOversized {
		encoding = "none"
		content = ""
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		Type     string `json:"type"`
		Encoding string `json:"encoding"`
		Size     int    `json:"size"`
		Name     string `json:"name"`
		Path     string `json:"path"`
		SHA      string `json:"sha"`
		Content  string `json:"content"`
	}{
		Type:     "file",
		Encoding: encoding,
		Size:     len(entry.Content),
		Name:     path.Base(reqPath),
		Path:     reqPath,
		SHA:      entry.SHA,
		Content:  content,
	})
}

// writeContentsDirResponse writes a bare JSON array (not a wrapped object).
// go-github GetContents tries a single RepositoryContent first and only then
// a []*RepositoryContent; a bare array is required for directory recognition.
func writeContentsDirResponse(w http.ResponseWriter, dirPath string, entries []DirEntry) {
	type dirEntryJSON struct {
		Type string `json:"type"`
		Name string `json:"name"`
		Path string `json:"path"`
		SHA  string `json:"sha"`
		Size int    `json:"size"`
	}

	out := make([]dirEntryJSON, 0, len(entries))
	for _, e := range entries {
		out = append(out, dirEntryJSON{
			Type: e.Type,
			Name: e.Name,
			Path: path.Join(dirPath, e.Name),
			SHA:  e.SHA,
			Size: e.Size,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}
