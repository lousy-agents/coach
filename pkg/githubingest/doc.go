// Package githubingest reads individual file contents from a GitHub
// repository via the GitHub Contents API, authenticated as a GitHub App
// installation.
//
// This package is optional and separately packaged from pkg/semantics:
// pkg/githubingest depends on github.com/google/go-github and
// github.com/bradleyfalzon/ghinstallation/v2, and pkg/semantics never
// imports pkg/githubingest or these dependencies. Consumers that only
// analyze raw source bytes never need to build or vendor a GitHub client.
//
// ReadFile issues two Contents API requests per call: the direct file fetch,
// plus a listing of the file's parent directory so it can detect an in-repo
// symlink that GitHub's Contents API would otherwise resolve transparently
// and report as a plain file (see reader.go's rejectIfPathIsSymlink). The
// second request lists only the immediate parent directory, not a
// whole-repository tree, so its cost does not scale with repository size.
//
// That directory listing is itself capped at GitHub's documented limit of
// 1,000 entries per directory: in a parent directory larger than that, a
// symlink entry can be absent from the (truncated) listing and pass the
// check undetected, unlike the Git Trees API, which at least reports
// truncation via a Truncated flag -- the Contents API listing gives no such
// signal. This is a narrower version of the same class of blind spot as a
// whole-repository tree walk would have (per-directory, 1,000+ files,
// rather than whole-repo size), and considered acceptable for v1.
package githubingest
