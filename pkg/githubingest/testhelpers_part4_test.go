// AC-5.10: every test in this package runs fully offline. Network access
// and real GitHub credentials are replaced throughout by
// generateTestRSAPrivateKeyPEM (a locally generated RSA key) and
// fakeGitHubTransport (a canned http.RoundTripper standing in for the
// GitHub API and the ghinstallation token-mint endpoint).
package githubingest_test

import (
	"io"
	"net/http"
	"strings"
)

// jsonResponse builds a canned *http.Response carrying body as its JSON
// payload with the given status code.
func jsonResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		Status:     http.StatusText(status),
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}
}
