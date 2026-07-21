package fakegithub

import (
	"sync"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// DefaultFixtureID is the FixtureID a caller can use when a test doesn't
// care about distinguishing between multiple fixture files/datasets.
const DefaultFixtureID = "coach-fakegithub-default"

// Scenario names a deterministic outcome a fixture entry selects, beyond
// its normal successful response, so a test can prove the fake's behavior
// for not-found/auth-failure/transient/oversized cases deterministically
// -- without depending on real GitHub state or timing.
type Scenario string

const (
	// ScenarioOK is the normal, successful outcome for the entry it's
	// attached to.
	ScenarioOK Scenario = "ok"
	// ScenarioNotFound makes the entry it's attached to behave as if the
	// resource does not exist (HTTP 404), regardless of any request field
	// that would otherwise resolve it.
	ScenarioNotFound Scenario = "not-found"
	// ScenarioAuthFail makes the entry it's attached to behave as an
	// authentication/authorization failure (HTTP 401), regardless of
	// whether the presented credential would otherwise be considered
	// valid.
	ScenarioAuthFail Scenario = "auth-failure"
	// ScenarioTransient makes the entry it's attached to behave as a
	// transient upstream failure (HTTP 503), so a caller can exercise
	// retry/backoff behavior deterministically.
	ScenarioTransient Scenario = "transient"
	// ScenarioOversized makes the entry it's attached to behave as a file
	// exceeding the GitHub Contents API's inline-content size limit,
	// mirroring real GitHub's encoding:"none" response shape.
	ScenarioOversized Scenario = "oversized"
)

// Identity is a fake GitHub user identity an OAuth flow can resolve to.
type Identity struct {
	ID    int64
	Login string
}

// OAuthCodeEntry is a fixture-registered authorization code awaiting
// exchange for an access token.
type OAuthCodeEntry struct {
	IdentityLogin string
	Scenario      Scenario
}

// OAuthTokenEntry is a fixture-registered OAuth access token, resolvable by
// GET /user.
type OAuthTokenEntry struct {
	IdentityLogin string
	Scenario      Scenario
}

// OAuthFixture is the OAuth-flow slice of a Fixture: client credentials,
// known identities, authorization codes awaiting exchange, and issued
// access tokens.
type OAuthFixture struct {
	ClientID     string
	ClientSecret string
	Identities   map[string]Identity        // login -> Identity
	Codes        map[string]OAuthCodeEntry  // code -> entry (consumed on exchange, see NOTE below)
	Tokens       map[string]OAuthTokenEntry // access token -> entry
}

// NOTE on "consumed on exchange": real GitHub authorization codes are
// single-use. The fake models this the simplest way that's still faithful
// to that contract: a successful exchange deletes the code from Codes (and,
// symmetrically, the handler that performs the exchange is responsible for
// registering the newly minted token into Tokens) rather than the fake
// tracking a separate "already used" flag per code. This is a deliberate,
// documented simplification -- not an oversight.

// InstallationEntry is a fixture-registered GitHub App installation: the
// installation token an access-token mint should return, and the scenario
// controlling that mint's outcome.
type InstallationEntry struct {
	Token    string
	Scenario Scenario
}

// RepoInstallationEntry maps a repository to the installation that governs
// it, for the repos/{owner}/{repo}/installation resolution endpoint.
type RepoInstallationEntry struct {
	InstallationID int64
	Scenario       Scenario
}

// PermissionEntry is a fixture-registered effective permission level for a
// (repo, username) pair.
type PermissionEntry struct {
	Level    string // "admin" | "write" | "read" | "none"
	Scenario Scenario
}

// InstallationFixture is the GitHub App installation/authorization slice of
// a Fixture: known installations, repo-to-installation mappings, and
// effective permissions.
type InstallationFixture struct {
	Installations map[int64]InstallationEntry      // installation ID -> entry
	RepoMappings  map[string]RepoInstallationEntry // "owner/repo" -> entry
	Permissions   map[string]PermissionEntry       // "owner/repo/username" -> entry
}

// FileEntry is a fixture-registered file's content and metadata, keyed
// within ContentsFixture.Files by "owner/repo/ref/path".
type FileEntry struct {
	Content  []byte
	SHA      string
	Scenario Scenario
}

// DirEntry is one entry within a fixture-registered directory listing.
type DirEntry struct {
	Name string
	Type string // "file" | "dir" | "symlink" | "submodule"
	SHA  string
	Size int
}

// ContentsFixture is the repository-content-read slice of a Fixture: file
// contents and directory listings, both keyed by repo/ref/path so a single
// fixture can answer both the file lookup and the parent-directory listing
// lookup pkg/githubingest.GitHubFileReader.ReadFile issues for symlink
// detection.
type ContentsFixture struct {
	Files map[string]FileEntry  // "owner/repo/ref/path" -> file entry
	Dirs  map[string][]DirEntry // "owner/repo/ref/dirpath" ("" = repo root) -> listing
}

// Fixture is the complete, versioned dataset a Server answers requests
// from: OAuth, GitHub App installation/authorization, and repository
// content reads, all stamped with the shared acceptanceharness fixture
// envelope.
type Fixture struct {
	Header       acceptanceharness.FixtureHeader
	OAuth        OAuthFixture
	Installation InstallationFixture
	Contents     ContentsFixture

	// mu guards runtime access to OAuth.Tokens/OAuth.Codes, the only
	// fixture maps mutated after construction (by oauthTokenHandler on a
	// successful code exchange) -- every other fixture map is written once
	// by test/fixture-author code before NewServer starts and never
	// mutated again, so it needs no synchronization here.
	mu sync.Mutex
}

// NewFixture returns an empty Fixture stamped with the current
// acceptanceharness.FixtureSchemaVersion, with every map initialized
// (never nil), so callers can populate it via plain map assignment without
// a nil-map panic.
func NewFixture(fixtureID string) Fixture {
	return Fixture{
		Header: acceptanceharness.NewFixtureHeader(fixtureID),
		OAuth: OAuthFixture{
			Identities: make(map[string]Identity),
			Codes:      make(map[string]OAuthCodeEntry),
			Tokens:     make(map[string]OAuthTokenEntry),
		},
		Installation: InstallationFixture{
			Installations: make(map[int64]InstallationEntry),
			RepoMappings:  make(map[string]RepoInstallationEntry),
			Permissions:   make(map[string]PermissionEntry),
		},
		Contents: ContentsFixture{
			Files: make(map[string]FileEntry),
			Dirs:  make(map[string][]DirEntry),
		},
	}
}

// TokenKind classifies which fixture-issued credential registry a bearer
// token belongs to, so handlers can detect and record misuse (e.g. an
// OAuth token presented on a repository endpoint) as
// acceptanceharness.AuthModeRejected rather than silently accepting it.
type TokenKind int

const (
	// TokenUnknown means token matches neither registry.
	TokenUnknown TokenKind = iota
	// TokenOAuth means token is a registered OAuth access token.
	TokenOAuth
	// TokenInstallation means token is a registered GitHub App
	// installation token.
	TokenInstallation
)

// ClassifyToken reports which fixture-issued credential registry token
// belongs to, if any. An empty token never classifies as anything other
// than TokenUnknown, even if a fixture-registered InstallationEntry has an
// empty Token field (as auth-fail/transient entries deliberately do, since
// those scenarios never successfully mint a real token).
func (f *Fixture) ClassifyToken(token string) TokenKind {
	if token == "" {
		return TokenUnknown
	}
	f.mu.Lock()
	_, ok := f.OAuth.Tokens[token]
	f.mu.Unlock()
	if ok {
		return TokenOAuth
	}
	for _, entry := range f.Installation.Installations {
		if entry.Token == token {
			return TokenInstallation
		}
	}
	return TokenUnknown
}
