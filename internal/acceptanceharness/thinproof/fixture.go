// Package thinproof is the shared, data-as-code fixture for GitHub issue
// #79's Task 0.3 thin offline proof: a single small file, at a fixed
// owner/repo/ref/path, served by internal/fakegithub and read back through
// pkg/githubingest's public GitHubFileReader API. Both the fake-GitHub side
// and the ingestion/runner side of the eventual Compose proof import this
// package directly, so the two processes always agree on the fixture without
// any file-based coordination (e.g. a JSON fixture file one process writes
// and the other reads).
package thinproof

import (
	"path"

	"github.com/lousy-agents/coach/internal/fakegithub"
)

// FixtureID identifies the thinproof dataset within acceptanceharness's
// shared fixture envelope (acceptanceharness.FixtureHeader.FixtureID).
const FixtureID = "coach-thinproof-v1"

// AppID, InstallationID, Owner, Repo, Ref, and Path identify the GitHub App
// installation and the single file this fixture serves.
const (
	AppID          int64 = 900100
	InstallationID int64 = 900200
	Owner                = "lousy-agents"
	Repo                 = "thinproof-fixture"
	Ref                  = "main"
	Path                 = "hello/greeting.go"
)

// installationToken is the bearer credential fakegithub.Handler mints and
// returns for InstallationID. Its literal value is a fixed placeholder --
// the real runner receives it dynamically over HTTP during Task 0.3's
// Compose proof, so callers of BuildFixture never need to know it in
// advance.
const installationToken = "thinproof-installation-token"

// fileContentSource is a small, syntactically valid, clean Go source file
// deliberately free of anything internal/codesignal's existing rules (in
// particular the hidden-input-mutation rule) would flag, so the eventual
// CodeSignal report golden for this fixture stays simple.
const fileContentSource = `package greeting

// Greet returns a friendly greeting for name.
func Greet(name string) string {
	return "Hello, " + name + "!"
}
`

// FileContent is the fixture file's exact byte content, returned verbatim by
// a successful pkg/githubingest.ReadFile against it.
var FileContent = []byte(fileContentSource)

// FileSHA is a stable, made-up SHA-like string for the fixture file. It is
// not a real git blob SHA -- the fake never validates it -- but it is
// 40 hex characters, matching the shape a real GitHub Contents API response
// would use.
const FileSHA = "1234567890abcdef1234567890abcdef12345678"

// BuildFixture returns the fakegithub.Fixture backing the thin offline
// proof: one GitHub App installation (InstallationID) mapped to
// Owner/Repo, and one file (Path) at Ref plus its parent directory listing
// (pkg/githubingest.ReadFile also lists the parent directory to reject
// symlinks; omitting that listing entry would 404 the read even though the
// file entry is present).
//
// The final value is reassembled field-by-field (rather than returned
// directly from a fakegithub.NewFixture-initialized local variable) so the
// unexported sync.Mutex embedded in fakegithub.Fixture is never copied --
// go vet's copylocks check rejects returning a Fixture-typed variable by
// value once any of its fields have been mutated in place.
func BuildFixture() fakegithub.Fixture {
	fx := fakegithub.NewFixture(FixtureID)

	fx.Installation.Installations[InstallationID] = fakegithub.InstallationEntry{
		Token:    installationToken,
		Scenario: fakegithub.ScenarioOK,
	}
	fx.Installation.RepoMappings[Owner+"/"+Repo] = fakegithub.RepoInstallationEntry{
		InstallationID: InstallationID,
		Scenario:       fakegithub.ScenarioOK,
	}

	dir := path.Dir(Path)
	fileKey := contentsKey(Owner, Repo, Ref, Path)
	dirKey := contentsKey(Owner, Repo, Ref, dir)

	fx.Contents.Files[fileKey] = fakegithub.FileEntry{
		Content:  FileContent,
		SHA:      FileSHA,
		Scenario: fakegithub.ScenarioOK,
	}
	fx.Contents.Dirs[dirKey] = []fakegithub.DirEntry{
		{Name: path.Base(Path), Type: "file", SHA: FileSHA, Size: len(FileContent)},
	}

	return fakegithub.Fixture{
		Header:         fx.Header,
		OAuth:          fx.OAuth,
		Installation:   fx.Installation,
		Contents:       fx.Contents,
		RejectedTokens: fx.RejectedTokens,
	}
}

// contentsKey builds the "owner/repo/ref/path" lookup key
// internal/fakegithub's contentsHandler uses for both Contents.Files and
// Contents.Dirs (see internal/fakegithub/contents.go's contentsKey), so this
// fixture's keys match byte-exact without depending on that unexported
// function directly.
func contentsKey(owner, repo, ref, p string) string {
	key := owner + "/" + repo + "/" + ref
	if p != "" && p != "." {
		key += "/" + p
	}
	return key
}
