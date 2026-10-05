package githubingest_test

import (
	"fmt"
	"net/http"
	"strings"
)

// tokenMintFailureTransport answers the ghinstallation token-mint request
// with the configured status and never expects to see a contents request,
// since a failed token mint should short-circuit ReadFile before go-github
// gets a chance to make the real Contents API call.
type tokenMintFailureTransport struct {
	status int
}

func (f *tokenMintFailureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.HasSuffix(req.URL.Path, "/access_tokens") {
		panic(fmt.Sprintf("unexpected contents request %s after a failed token mint", req.URL))
	}
	return jsonResponse(req, f.status, `{"message":"Bad credentials"}`), nil
}
