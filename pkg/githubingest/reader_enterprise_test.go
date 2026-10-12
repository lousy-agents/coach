package githubingest_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// AC-5.3 regression: go-github's WithEnterpriseURLs and ghinstallation's
// Transport.BaseURL normalize a bare Enterprise host differently -- go-github
// appends "api/v3/" to the path, ghinstallation does not. Passing the raw
// caller-supplied BaseURL straight to ghinstallation (as the code previously
// did) sent the installation-token-mint request to the wrong path
// (".../app/installations/.../access_tokens" instead of
// ".../api/v3/app/installations/.../access_tokens"), so real Enterprise auth
// would fail before ReadFile ever reached the contents endpoint, even though
// the contents request URL alone looked correct.
func TestReadFile_EnterpriseBaseURLNormalizesTokenAndContentsRequestsToSameAPIBase(t *testing.T) {
	reader, transport := givenEnterpriseReader(t, "https://ghe.example.com/")
	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "hello.txt"}

	whenReadFileSucceeds(t, reader, ref)

	if len(transport.seen) != 3 {
		t.Fatalf("ReadFile(%+v): got %d outbound requests %v, want exactly 3 (token mint, contents, then the AC-5.7 directory-listing symlink check)", ref, len(transport.seen), transport.seen)
	}
	tokenURL, contentsURL, dirListingURL := transport.seen[0], transport.seen[1], transport.seen[2]

	const wantAPIBase = "https://ghe.example.com/api/v3/"
	thenRequestTargets(t, "token-mint", tokenURL, wantAPIBase, "")
	thenRequestTargets(t, "contents", contentsURL, wantAPIBase, "/repos/acme/widgets/contents/hello.txt?ref=main")
	thenRequestTargets(t, "directory listing", dirListingURL, wantAPIBase, "?ref=main")
}

// givenEnterpriseReader builds a reader configured with baseURL as its
// GitHub Enterprise BaseURL, wired to a transport that records every
// outbound request URL for inspection.
func givenEnterpriseReader(t *testing.T, baseURL string) (*githubingest.GitHubFileReader, *urlRecordingEnterpriseTransport) {
	t.Helper()

	transport := &urlRecordingEnterpriseTransport{}
	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          1,
		InstallationID: 2,
		PrivateKey:     generateTestRSAPrivateKeyPEM(t),
		BaseURL:        baseURL,
		Transport:      transport,
	})
	if err != nil {
		t.Fatalf("NewGitHubFileReader: unexpected error: %v", err)
	}
	return reader, transport
}

// whenReadFileSucceeds calls ReadFile and fails the test immediately if it
// returns an error, since the tests using it assert on requests recorded
// during a successful call, not on error-path behavior.
func whenReadFileSucceeds(t *testing.T, reader *githubingest.GitHubFileReader, ref githubingest.GitHubFileRef) {
	t.Helper()
	if _, _, err := reader.ReadFile(context.Background(), ref); err != nil {
		t.Fatalf("ReadFile(%+v): unexpected error: %v", ref, err)
	}
}

// thenRequestTargets fails the test unless url starts with wantAPIBase and
// (when wantSuffix is non-empty) ends with wantSuffix.
func thenRequestTargets(t *testing.T, name, url, wantAPIBase, wantSuffix string) {
	t.Helper()
	if !strings.HasPrefix(url, wantAPIBase) {
		t.Fatalf("%s request URL: got %q, want it to start with %q (the same normalized Enterprise API base go-github uses)", name, url, wantAPIBase)
	}
	if wantSuffix != "" && !strings.HasSuffix(url, wantSuffix) {
		t.Fatalf("%s request URL: got %q, want it to end with %q", name, url, wantSuffix)
	}
}
