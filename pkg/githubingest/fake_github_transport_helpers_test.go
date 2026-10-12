package githubingest_test

import (
	"net/http"
	"strings"
)

// contentsHandlerFunc builds the canned *http.Response for a Contents API
// call in a fake transport. It never touches the network.
type contentsHandlerFunc func(req *http.Request) *http.Response

// fakeGitHubTransport is an offline http.RoundTripper stand-in for GitHub's
// API. It answers the ghinstallation installation-token mint request with a
// canned token, and delegates every Contents API call -- both the direct
// file request and the parent-directory listing ReadFile uses to detect
// symlinks (AC-5.7) -- to handleContents, so a test can distinguish them by
// inspecting req.URL.Path when it needs to.
type fakeGitHubTransport struct {
	handleContents contentsHandlerFunc
}

func (f *fakeGitHubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Path, "/access_tokens") {
		return mintInstallationTokenResponse(req), nil
	}
	return f.handleContents(req), nil
}
