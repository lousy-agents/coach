package fakegithub

import (
	"net/http"
	"strings"
)

// extractBearerToken returns the credential from Authorization using the
// "token" or "Bearer" scheme, or "" if neither is present.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	for _, scheme := range []string{"token ", "Bearer "} {
		if strings.HasPrefix(auth, scheme) {
			return strings.TrimPrefix(auth, scheme)
		}
	}
	return ""
}
