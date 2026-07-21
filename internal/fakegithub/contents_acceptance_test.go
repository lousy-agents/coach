package fakegithub_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

// oversizedContent is larger than the Contents API's 1 MiB inline-content
// limit (see pkg/githubingest's maxContentSize), so a fixture File entry
// built from it exercises the ScenarioOversized path end to end.
var oversizedContent = make([]byte, 1<<20+1)

// contentsFixtureRSAKey returns a freshly generated RSA private key,
// PKCS#1-PEM-encoded the same way GitHub encodes App private keys it
// issues. Ginkgo-local (mirrors pkg/githubingest's own ginkgoRSAKey, which
// exists for the same reason: a Ginkgo spec has no *testing.T, and
// GinkgoT() alone does not satisfy testing.TB).
func contentsFixtureRSAKey() []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block)
}

// newContentsFixture builds a Fixture covering every scenario this file
// exercises via pkg/githubingest.GitHubFileReader.ReadFile's public API: a
// registered installation (so ghinstallation's automatic token mint against
// this same fake server succeeds), a small readable file plus its parent
// directory listing, an oversized file, and dedicated not-found/
// auth-failure/transient file entries.
func newContentsFixture() *fakegithub.Fixture {
	fx := fakegithub.NewFixture("contents-fixture")

	fx.Installation.Installations[42] = fakegithub.InstallationEntry{Token: "contents-installation-token", Scenario: fakegithub.ScenarioOK}
	fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{InstallationID: 42, Scenario: fakegithub.ScenarioOK}

	fx.Contents.Files["acme/widgets/main/dir/hello.txt"] = fakegithub.FileEntry{
		Content:  []byte("hello world"),
		SHA:      "abc123sha",
		Scenario: fakegithub.ScenarioOK,
	}
	fx.Contents.Files["acme/widgets/main/dir/big.bin"] = fakegithub.FileEntry{
		Content:  oversizedContent,
		SHA:      "bigsha",
		Scenario: fakegithub.ScenarioOversized,
	}
	fx.Contents.Dirs["acme/widgets/main/dir"] = []fakegithub.DirEntry{
		{Name: "hello.txt", Type: "file", SHA: "abc123sha", Size: len("hello world")},
		{Name: "big.bin", Type: "file", SHA: "bigsha", Size: len(oversizedContent)},
	}

	fx.Contents.Files["acme/widgets/main/dir/authfail.txt"] = fakegithub.FileEntry{Scenario: fakegithub.ScenarioAuthFail}
	fx.Contents.Files["acme/widgets/main/dir/transient.txt"] = fakegithub.FileEntry{Scenario: fakegithub.ScenarioTransient}
	// "dir/missing.txt" is deliberately never registered in Files, modeling
	// ScenarioNotFound as the natural absence of a fixture entry.

	return &fx
}

func newContentsReader(server *fakegithub.Server) *githubingest.GitHubFileReader {
	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          12345,
		InstallationID: 42,
		PrivateKey:     contentsFixtureRSAKey(),
		BaseURL:        server.URL(),
	})
	Expect(err).NotTo(HaveOccurred())
	return reader
}

var _ = Describe("fake GitHub repository content reads, via pkg/githubingest's public API", func() {
	var (
		fx     *fakegithub.Fixture
		server *fakegithub.Server
		reader *githubingest.GitHubFileReader
	)

	BeforeEach(func() {
		fx = newContentsFixture()
		server = fakegithub.NewServer(fx)
		reader = newContentsReader(server)
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when the file exists and is within the size limit (ScenarioOK)", func() {
		It("returns the decoded bytes and metadata, and records both the file read and the parent-directory listing with AuthModeInstallation", func() {
			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/hello.txt"}
			data, meta, err := reader.ReadFile(context.Background(), ref)

			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("hello world"))
			Expect(meta).To(Equal(githubingest.FileMetadata{Path: "dir/hello.txt", Ref: "main", SHA: "abc123sha", Size: len("hello world")}))

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			Expect(records[0].FixtureID).To(Equal("contents-fixture"))

			var sawFileRead, sawParentDirListing bool
			for _, rec := range records {
				if rec.AuthMode != acceptanceharness.AuthModeInstallation || rec.Method != http.MethodGet {
					continue
				}
				if strings.HasSuffix(rec.Path, "/contents/dir/hello.txt") {
					sawFileRead = true
				}
				if strings.HasSuffix(rec.Path, "/contents/dir") {
					sawParentDirListing = true
				}
			}
			Expect(sawFileRead).To(BeTrue(), "expected a recorded file contents GET, got %+v", records)
			Expect(sawParentDirListing).To(BeTrue(), "expected a recorded parent-directory listing GET (symlink check), got %+v", records)
		})
	})

	Context("when the parent directory listing marks the path as a symlink", func() {
		It("returns githubingest.ErrUnsupportedContent via ReadFile's public API", func() {
			fx.Contents.Files["acme/widgets/main/dir/link.txt"] = fakegithub.FileEntry{
				Content:  []byte("resolved target bytes"),
				SHA:      "linksha",
				Scenario: fakegithub.ScenarioOK,
			}
			fx.Contents.Dirs["acme/widgets/main/dir"] = []fakegithub.DirEntry{
				{Name: "hello.txt", Type: "file", SHA: "abc123sha", Size: len("hello world")},
				{Name: "big.bin", Type: "file", SHA: "bigsha", Size: len(oversizedContent)},
				{Name: "link.txt", Type: "symlink", SHA: "linksha", Size: 0},
			}

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/link.txt"}
			data, _, err := reader.ReadFile(context.Background(), ref)

			Expect(errors.Is(err, githubingest.ErrUnsupportedContent)).To(BeTrue(), "got err %v, want errors.Is(err, ErrUnsupportedContent)", err)
			Expect(data).To(BeNil())
		})
	})

	Context("when the file exceeds the Contents API's size limit (ScenarioOversized)", func() {
		It("returns githubingest.ErrTooLarge and no bytes", func() {
			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/big.bin"}
			data, _, err := reader.ReadFile(context.Background(), ref)

			Expect(errors.Is(err, githubingest.ErrTooLarge)).To(BeTrue(), "got err %v, want errors.Is(err, ErrTooLarge)", err)
			Expect(data).To(BeNil())
		})
	})

	Context("when the file was never registered in the fixture (natural ScenarioNotFound)", func() {
		It("returns githubingest.ErrNotFound", func() {
			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/missing.txt"}
			_, _, err := reader.ReadFile(context.Background(), ref)

			Expect(errors.Is(err, githubingest.ErrNotFound)).To(BeTrue(), "got err %v, want errors.Is(err, ErrNotFound)", err)
		})
	})

	Context("when the file is registered as ScenarioAuthFail", func() {
		It("returns githubingest.ErrAuth, and the failure comes from the contents handler itself (not from an earlier token-mint failure)", func() {
			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/authfail.txt"}
			_, _, err := reader.ReadFile(context.Background(), ref)

			Expect(errors.Is(err, githubingest.ErrAuth)).To(BeTrue(), "got err %v, want errors.Is(err, ErrAuth)", err)

			// The above assertion alone can't distinguish "the contents
			// handler rejected the read" from "token mint failed before the
			// contents handler was ever reached" -- both surface as
			// githubingest.ErrAuth from ReadFile's perspective. Require a
			// recorded request against the contents-read path itself, with
			// this scenario, so the spec can only pass once the real
			// contents handler (not the installation-token-mint stub)
			// produces the auth failure.
			var sawContentsAuthFail bool
			for _, rec := range server.Recorder().Records() {
				if rec.Method == http.MethodGet && strings.Contains(rec.Path, "/contents/") && rec.Scenario == string(fakegithub.ScenarioAuthFail) {
					sawContentsAuthFail = true
				}
			}
			Expect(sawContentsAuthFail).To(BeTrue(), "expected a recorded GET request against a /contents/ path with scenario %q, got %+v", fakegithub.ScenarioAuthFail, server.Recorder().Records())
		})
	})

	Context("when the file is registered as ScenarioTransient", func() {
		It("returns a non-nil error that matches none of githubingest's documented sentinels", func() {
			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/transient.txt"}
			_, _, err := reader.ReadFile(context.Background(), ref)

			Expect(err).To(HaveOccurred())
			for _, sentinel := range []error{
				githubingest.ErrAuth,
				githubingest.ErrNotFound,
				githubingest.ErrUnsupportedContent,
				githubingest.ErrEmptyContent,
				githubingest.ErrTooLarge,
			} {
				Expect(errors.Is(err, sentinel)).To(BeFalse(), "err %v unexpectedly matched sentinel %v", err, sentinel)
			}
		})
	})
})
