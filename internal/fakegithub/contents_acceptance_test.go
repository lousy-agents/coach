package fakegithub_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

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
			body_contentsAcceptanceTest_returnsTheDecodedBytesAndMetadataAndRecordsBothT_39(server, reader)
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
				{Name: "big.bin", Type: "file", SHA: "bigsha", Size: 1<<20 + 1},
				{Name: "link.txt", Type: "symlink", SHA: "linksha", Size: 0},
			}

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/link.txt"}
			data, _, err := reader.ReadFile(context.Background(), ref)

			Expect(errors.Is(err, githubingest.ErrUnsupportedContent)).To(BeTrue(), "got err %v, want errors.Is(err, ErrUnsupportedContent)", err)
			Expect(data).To(BeNil())
		})
	})

	Context("when the file exceeds the Contents API's size limit (ScenarioOversized)", func() {
		It("returns githubingest.ErrTooLarge and no bytes, without requiring oversized Content bytes in the fixture", func() {
			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/big.bin"}
			data, _, err := reader.ReadFile(context.Background(), ref)

			Expect(errors.Is(err, githubingest.ErrTooLarge)).To(BeTrue(), "got err %v, want errors.Is(err, ErrTooLarge)", err)
			Expect(data).To(BeNil())
		})
	})

	Context("when the file is registered with an unknown Scenario", func() {
		It("fails loud with HTTP 500 rather than succeeding with empty content", func() {
			req, err := http.NewRequest(http.MethodGet, server.URL()+"/api/v3/repos/acme/widgets/contents/dir/typo-scenario.txt?ref=main", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "token contents-installation-token")

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			Expect(resp.Header.Get("Content-Type")).To(HavePrefix("application/json"))
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
			body_contentsAcceptanceTest_returnsGithubingestErrAuthAndTheFailureComesFrom_124(server, reader)
		})
	})

	Context("when the file is registered as ScenarioTransient", func() {
		It("returns a non-nil error that matches none of githubingest's documented sentinels", func() {
			body_contentsAcceptanceTest_returnsANonNilErrorThatMatchesNoneOfGithubingest_143(reader)
		})
	})
})

func contentsFixtureRSAKey() []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block)
}

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
		SHA:      "bigsha",
		Scenario: fakegithub.ScenarioOversized,
	}
	fx.Contents.Dirs["acme/widgets/main/dir"] = []fakegithub.DirEntry{
		{Name: "hello.txt", Type: "file", SHA: "abc123sha", Size: len("hello world")},
		{Name: "big.bin", Type: "file", SHA: "bigsha", Size: 1<<20 + 1},
	}

	fx.Contents.Files["acme/widgets/main/dir/authfail.txt"] = fakegithub.FileEntry{Scenario: fakegithub.ScenarioAuthFail}
	fx.Contents.Files["acme/widgets/main/dir/transient.txt"] = fakegithub.FileEntry{Scenario: fakegithub.ScenarioTransient}
	fx.Contents.Files["acme/widgets/main/dir/typo-scenario.txt"] = fakegithub.FileEntry{Scenario: "ok "}

	return &fx
}
