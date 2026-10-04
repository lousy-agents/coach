package fakegithub_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"

	"github.com/google/go-github/v92/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

const (
	contractAppID          int64 = 12345
	contractInstallationID int64 = 99
	contractInstallToken         = "gogithub-contract-install-token"
)

var _ = Describe("fakegithub go-github client contract", func() {
	var (
		server *fakegithub.Server
		ctx    context.Context
	)

	BeforeEach(func() {
		server = fakegithub.NewServer(newContractFixture())
		ctx = context.Background()
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("Apps API (App JWT via ghinstallation.AppsTransport)", func() {
		It("mints an installation token with CreateInstallationToken", func() {
			client := newAppsClient(server)

			token, resp, err := client.Apps.CreateInstallationToken(ctx, contractInstallationID, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(token.GetToken()).To(Equal(contractInstallToken))
			Expect(token.GetExpiresAt().Time.IsZero()).To(BeFalse(), "expires_at must decode for ghinstallation")

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			last := records[len(records)-1]
			Expect(last.Method).To(Equal(http.MethodPost))
			Expect(last.Path).To(ContainSubstring("/app/installations/"))
			Expect(last.Path).To(ContainSubstring("/access_tokens"))
			Expect(last.FixtureID).To(Equal("gogithub-contract-fixture"))
		})

		It("resolves a repository installation with GetRepositoryInstallation", func() {
			client := newAppsClient(server)

			inst, resp, err := client.Apps.GetRepositoryInstallation(ctx, "acme", "widgets")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(inst.GetID()).To(Equal(contractInstallationID))
		})

		It("surfaces not-found installation mint as a go-github error", func() {
			client := newAppsClient(server)

			_, resp, err := client.Apps.CreateInstallationToken(ctx, 999999, nil)
			Expect(err).To(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("surfaces unmapped repository installation as a go-github error", func() {
			client := newAppsClient(server)

			_, resp, err := client.Apps.GetRepositoryInstallation(ctx, "acme", "missing")
			Expect(err).To(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Describe("Repositories API (installation token via ghinstallation.Transport)", func() {
		It("reads collaborator permission with GetPermissionLevel", func() {
			body_gogithubContractAcceptanceTest_readsCollaboratorPermissionWithGetPermissionLeve_89(server, ctx)
		})

		It("reads a file with GetContents", func() {
			client := newInstallationClient(server)

			file, dir, resp, err := client.Repositories.GetContents(ctx, "acme", "widgets", "src/main.go", &github.RepositoryContentGetOptions{Ref: "main"})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(dir).To(BeNil())
			Expect(file).NotTo(BeNil())
			Expect(file.GetType()).To(Equal("file"))
			Expect(file.GetSHA()).To(Equal("contract-sha"))
			content, err := file.GetContent()
			Expect(err).NotTo(HaveOccurred())
			Expect(content).To(Equal("package main\n"))
		})

		It("lists a directory with GetContents", func() {
			client := newInstallationClient(server)

			file, dir, resp, err := client.Repositories.GetContents(ctx, "acme", "widgets", "src", &github.RepositoryContentGetOptions{Ref: "main"})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(file).To(BeNil())
			Expect(dir).NotTo(BeEmpty())
			Expect(dir[0].GetName()).To(Equal("main.go"))
			Expect(dir[0].GetType()).To(Equal("file"))
		})

		It("surfaces missing content as a go-github error", func() {
			client := newInstallationClient(server)

			_, _, resp, err := client.Repositories.GetContents(ctx, "acme", "widgets", "nope.go", &github.RepositoryContentGetOptions{Ref: "main"})
			Expect(err).To(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Describe("Users API (OAuth access token)", func() {
		It("resolves the authenticated user with Users.Get(\"\") under Enterprise BaseURL", func() {
			client := newOAuthClient(server, "contract-oauth-token")

			user, resp, err := client.Users.Get(ctx, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(user.GetLogin()).To(Equal("octocat"))
			Expect(user.GetID()).To(Equal(int64(1)))

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			last := records[len(records)-1]
			Expect(last.AuthMode).To(Equal(acceptanceharness.AuthModeOAuth))
			Expect(last.Path).To(Equal("/api/v3/user"))
		})
	})

	Describe("end-to-end App JWT → install token → contents", func() {
		It("mints via Apps API then reads contents with that token through go-github", func() {
			body_gogithubContractAcceptanceTest_mintsViaAppsAPIThenReadsContentsWithThatTokenThr_164(server, ctx)
		})
	})
})

func contractRSAKey() []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

func newContractFixture() *fakegithub.Fixture {
	fx := fakegithub.NewFixture("gogithub-contract-fixture")
	fx.OAuth.ClientID = "contract-client-id"
	fx.OAuth.ClientSecret = "contract-client-secret"
	fx.OAuth.Identities["octocat"] = fakegithub.Identity{ID: 1, Login: "octocat"}
	fx.OAuth.Tokens["contract-oauth-token"] = fakegithub.OAuthTokenEntry{
		IdentityLogin: "octocat",
		Scenario:      fakegithub.ScenarioOK,
	}

	fx.Installation.Installations[contractInstallationID] = fakegithub.InstallationEntry{
		Token:    contractInstallToken,
		Scenario: fakegithub.ScenarioOK,
	}
	fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{
		InstallationID: contractInstallationID,
		Scenario:       fakegithub.ScenarioOK,
	}
	fx.Installation.Permissions["acme/widgets/octocat"] = fakegithub.PermissionEntry{
		Level:    "write",
		Scenario: fakegithub.ScenarioOK,
	}

	fx.Contents.Files["acme/widgets/main/src/main.go"] = fakegithub.FileEntry{
		Content:  []byte("package main\n"),
		SHA:      "contract-sha",
		Scenario: fakegithub.ScenarioOK,
	}
	fx.Contents.Dirs["acme/widgets/main/src"] = []fakegithub.DirEntry{
		{Name: "main.go", Type: "file", SHA: "contract-sha", Size: len("package main\n")},
	}

	return &fx
}
