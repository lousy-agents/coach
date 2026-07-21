package fakegithub

import (
	"sync"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// DefaultFixtureID is a FixtureID for tests that do not need multiple datasets.
const DefaultFixtureID = "coach-fakegithub-default"

// Scenario names a deterministic non-success (or success) outcome for a
// fixture entry, so not-found/auth-failure/transient/oversized behavior is
// independent of real GitHub state or timing.
type Scenario string

const (
	ScenarioOK        Scenario = "ok"           // successful response
	ScenarioNotFound  Scenario = "not-found"    // HTTP 404
	ScenarioAuthFail  Scenario = "auth-failure" // HTTP 401
	ScenarioTransient Scenario = "transient"    // HTTP 503
	ScenarioOversized Scenario = "oversized"    // Contents API oversize (encoding "none")
)

// Identity is a fake GitHub user an OAuth flow can resolve to.
type Identity struct {
	ID    int64
	Login string
}

// OAuthCodeEntry is a fixture-registered authorization code awaiting exchange.
type OAuthCodeEntry struct {
	IdentityLogin string
	Scenario      Scenario
}

// OAuthTokenEntry is a fixture-registered OAuth access token for GET /user.
type OAuthTokenEntry struct {
	IdentityLogin string
	Scenario      Scenario
}

// OAuthFixture is the OAuth slice of a [Fixture].
type OAuthFixture struct {
	ClientID     string
	ClientSecret string
	Identities   map[string]Identity // login -> Identity
	// Codes maps authorization code -> entry. A successful exchange deletes
	// the code (single-use, like real GitHub); the handler registers the
	// minted token into Tokens. No separate "already used" flag.
	Codes  map[string]OAuthCodeEntry
	Tokens map[string]OAuthTokenEntry // access token -> entry
}

// InstallationEntry is a fixture-registered GitHub App installation.
type InstallationEntry struct {
	Token    string
	Scenario Scenario
}

// RepoInstallationEntry maps owner/repo to the governing installation ID.
type RepoInstallationEntry struct {
	InstallationID int64
	Scenario       Scenario
}

// PermissionEntry is the effective permission for a (repo, username) pair.
type PermissionEntry struct {
	Level    string // "admin" | "write" | "read" | "none"
	Scenario Scenario
}

// InstallationFixture is the GitHub App installation slice of a [Fixture].
type InstallationFixture struct {
	Installations map[int64]InstallationEntry      // installation ID -> entry
	RepoMappings  map[string]RepoInstallationEntry // "owner/repo" -> entry
	Permissions   map[string]PermissionEntry       // "owner/repo/username" -> entry
}

// FileEntry is file content keyed in [ContentsFixture.Files] by
// "owner/repo/ref/path". For ScenarioOversized, Content may be nil — the
// wire size is forced above the Contents API 1 MiB limit.
type FileEntry struct {
	Content  []byte
	SHA      string
	Scenario Scenario
}

// DirEntry is one entry in a fixture directory listing.
type DirEntry struct {
	Name string
	Type string // "file" | "dir" | "symlink" | "submodule"
	SHA  string
	Size int
}

// ContentsFixture holds file contents and directory listings, both keyed by
// repo/ref/path so one fixture can answer the file read and the parent-dir
// listing githubingest ReadFile uses for symlink detection.
//
// A successful ReadFile needs both: Files[fileKey] and Dirs[parentKey].
// Omitting the parent Dirs entry yields 404 on the symlink-check listing
// and surfaces as githubingest.ErrNotFound even when the file Scenario is OK.
type ContentsFixture struct {
	Files map[string]FileEntry  // "owner/repo/ref/path" -> file
	Dirs  map[string][]DirEntry // "owner/repo/ref/dirpath" (root path segment omitted) -> listing
}

// Fixture is the versioned dataset a [Server] answers from, stamped with the
// shared acceptanceharness fixture envelope.
type Fixture struct {
	Header       acceptanceharness.FixtureHeader
	OAuth        OAuthFixture
	Installation InstallationFixture
	Contents     ContentsFixture

	// RejectedTokens are bearer credentials that must never be accepted on
	// any route (AuthModeRejected). Use for non-GitHub credentials (notably a
	// Coach JWT stand-in) without inventing coach-api or weakening the
	// "unknown bearer ≈ unverifiable App JWT" simplification on installation
	// mint/resolution. Empty string is never a rejected token.
	RejectedTokens map[string]struct{}

	// mu guards OAuth.Tokens and OAuth.Codes, the only maps mutated after
	// construction (successful code exchange). All other maps are write-once
	// before NewServer and need no synchronization.
	mu sync.Mutex
}

// NewFixture returns an empty Fixture with SchemaVersion set and every map
// non-nil so callers can assign without a nil-map panic.
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
		RejectedTokens: make(map[string]struct{}),
	}
}

// TokenKind classifies which fixture credential registry a bearer token
// belongs to, so handlers can record misuse as AuthModeRejected instead of
// accepting it.
type TokenKind int

const (
	TokenUnknown      TokenKind = iota // neither registry
	TokenOAuth                         // registered OAuth access token
	TokenInstallation                  // registered installation token
	TokenRejected                      // Fixture.RejectedTokens (e.g. Coach JWT stand-in)
)

// ClassifyToken reports which credential registry token belongs to.
// Empty token is always [TokenUnknown], even if an InstallationEntry has an
// empty Token (auth-fail/transient entries never mint). RejectedTokens is
// checked before OAuth/installation so a rejected stand-in cannot collide
// with a minted credential and be reclassified.
func (f *Fixture) ClassifyToken(token string) TokenKind {
	if token == "" {
		return TokenUnknown
	}
	if _, ok := f.RejectedTokens[token]; ok {
		return TokenRejected
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
