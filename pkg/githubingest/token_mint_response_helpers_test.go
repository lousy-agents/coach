package githubingest_test

import (
	"io"
	"net/http"
	"strings"
)

// mintInstallationTokenResponse answers ghinstallation's installation access
// token exchange with a canned, never-expiring (for the test's short
// lifetime) token. No network access occurs.
func mintInstallationTokenResponse(req *http.Request) *http.Response {
	const body = `{"token":"test-installation-token","expires_at":"2999-01-01T00:00:00Z"}`
	return &http.Response{
		Status:     "201 Created",
		StatusCode: http.StatusCreated,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}
}
