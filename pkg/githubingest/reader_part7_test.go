package githubingest_test

import (
	"fmt"
	"net/http"

	"strings"
)

func (f *tokenMintFailureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.HasSuffix(req.URL.Path, "/access_tokens") {
		panic(fmt.Sprintf("unexpected contents request %s after a failed token mint", req.URL))
	}
	return jsonResponse(req, f.status, `{"message":"Bad credentials"}`), nil
}
