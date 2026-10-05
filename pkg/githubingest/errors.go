package githubingest

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v92/github"
)

// Sentinel errors returned by GitHubFileReader. Callers should use
// errors.Is to test for these.
var (
	// ErrAuth indicates the GitHub API rejected the request as unauthorized
	// or forbidden (HTTP 401/403).
	ErrAuth = errors.New("githubingest: authentication failed")

	// ErrNotFound indicates the requested path does not exist at the given
	// ref (HTTP 404).
	ErrNotFound = errors.New("githubingest: file not found")

	// ErrUnsupportedContent indicates the requested path resolved to
	// something other than a regular file (a directory, symlink, or
	// submodule).
	ErrUnsupportedContent = errors.New("githubingest: unsupported content type")

	// ErrEmptyContent indicates the file exists but decodes to zero bytes.
	ErrEmptyContent = errors.New("githubingest: file content is empty")

	// ErrTooLarge indicates the file exceeds the GitHub Contents API's
	// 1 MiB size limit.
	ErrTooLarge = errors.New("githubingest: file exceeds maximum supported size")
)

// mapContentsAPIError maps a failure from a GetContents-style call to the
// sentinel it represents:
//   - resp != nil (a genuine Contents API response): 404 -> ErrNotFound,
//     401/403 -> ErrAuth.
//   - resp == nil and the failure is a token-mint failure (see
//     isTokenMintFailure): always ErrAuth, regardless of the token-mint
//     endpoint's own status code. Minting a token is itself an
//     authentication step, so even a 404 there (e.g. an unknown or revoked
//     InstallationID) means "this GitHub App installation could not be
//     authenticated," never "this file doesn't exist" -- AC-5.5's
//     404 -> ErrNotFound scope is the Contents API's own response, not the
//     token-mint endpoint's.
//   - anything else: err wrapped with action for context, matching no
//     sentinel.
func mapContentsAPIError(err error, resp *github.Response, action string) error {
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return fmt.Errorf("githubingest: %s: %w", action, ErrNotFound)
		case http.StatusUnauthorized, http.StatusForbidden:
			return fmt.Errorf("githubingest: %s rejected with status %d: %w", action, resp.StatusCode, ErrAuth)
		}
	} else if isTokenMintFailure(err) {
		return fmt.Errorf("githubingest: authenticating GitHub App installation while %s: %w", action, ErrAuth)
	}
	return fmt.Errorf("githubingest: %s: %w", action, err)
}

// isTokenMintFailure reports whether err originates from a failed
// ghinstallation installation-token mint, as opposed to a genuine Contents
// API response. ghinstallation wraps any non-2xx token-mint response (401,
// 403, 404, ...) in *ghinstallation.HTTPError; when that happens, go-github
// never issues the underlying Contents API request at all, so the caller's
// *github.Response is nil and the real status code is only reachable
// through this wrapped error.
func isTokenMintFailure(err error) bool {
	var httpErr *ghinstallation.HTTPError
	return errors.As(err, &httpErr) && httpErr.Response != nil
}
