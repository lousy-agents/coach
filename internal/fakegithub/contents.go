package fakegithub

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"path"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// contentsHandler answers GET /api/v3/repos/{owner}/{repo}/contents/{path...}.
// pkg/githubingest.GitHubFileReader.ReadFile issues this same route for two
// distinct purposes: a file-content read, and (only after that first read
// succeeds and reports type "file") a parent-directory listing for symlink
// detection. This handler tells the two apart purely by fixture lookup:
// fx.Contents.Files wins if the request's key is registered there,
// otherwise fx.Contents.Dirs is tried, otherwise the key models a natural
// not-found (mirroring installationTokenHandler/permissionHandler's
// dispatch shape in installation.go).
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
			// The credential itself was valid and correctly used here; a
			// scenario-driven 404/401/503 outcome below is about the
			// requested resource, not the credential, so this always
			// records AuthModeInstallation.
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

// contentsKey builds the "owner/repo/ref/path" fixture-lookup key shared by
// fx.Contents.Files and fx.Contents.Dirs, matching how
// contents_acceptance_test.go's newContentsFixture registers entries (e.g.
// "acme/widgets/main/dir/hello.txt"). path.Join drops empty path segments
// and cleans the result, so a root-level path ("") collapses the key to
// "owner/repo/ref" with no trailing separator -- an edge case no acceptance
// spec exercises today, but one this keeps internally consistent between
// the file lookup and the parent-directory lookup for a root-level file.
func contentsKey(owner, repo, ref, p string) string {
	return path.Join(owner, repo, ref, p)
}

// writeContentsFileResponse writes entry as a single
// github.RepositoryContent-shaped JSON object (a "type":"file" entry),
// matching go-github's expected response shape for GET .../contents/{path}.
// A ScenarioOversized entry is written with encoding "none" and an empty
// content field, mirroring real GitHub's documented behavior for files
// exceeding the Contents API's inline-content size limit: pkg/githubingest
// checks size against that limit and returns ErrTooLarge before ever
// looking at the content field, so its exact value has no functional
// effect here -- and skipping the base64 encoding avoids needlessly
// serializing a multi-megabyte payload for a value nothing decodes.
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

// writeContentsDirResponse writes entries as a bare JSON array, never an
// object wrapping an array: go-github's GetContents tries to unmarshal the
// response body as a single RepositoryContent object first, only falling
// back to a []*RepositoryContent array on that failure, so a directory
// listing must be a bare array to be recognized as one at all.
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
