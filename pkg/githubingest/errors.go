package githubingest

import "errors"

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
