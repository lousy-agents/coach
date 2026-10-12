package githubingest_test

import (
	"net/http"
	"strings"
)

// urlRecordingEnterpriseTransport records every outbound request URL (in
// order) while answering both the ghinstallation token-mint request and the
// Contents API request, so a test can assert they share the same normalized
// Enterprise API base.
type urlRecordingEnterpriseTransport struct {
	seen []string
}

func (u *urlRecordingEnterpriseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u.seen = append(u.seen, req.URL.String())
	if strings.HasSuffix(req.URL.Path, "/access_tokens") {
		return mintInstallationTokenResponse(req), nil
	}
	// Both the direct file request and the AC-5.7 parent-directory listing
	// request get this same canned single-file response; parsed as a
	// directory listing it has no entries, so the symlink check finds
	// nothing and ReadFile proceeds -- exactly the "not a symlink" default
	// this test needs.
	const canned = `{
		"type": "file",
		"encoding": "base64",
		"size": 5,
		"name": "hello.txt",
		"path": "hello.txt",
		"sha": "deadbeef",
		"content": "aGVsbG8="
	}`
	return jsonResponse(req, http.StatusOK, canned), nil
}
