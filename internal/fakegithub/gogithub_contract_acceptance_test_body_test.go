package fakegithub_test

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/go-github/v92/github"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

func body_gogithubContractAcceptanceTest_readsCollaboratorPermissionWithGetPermissionLeve_89(server *fakegithub.Server, ctx context.Context) {
	client := newInstallationClient(server)

	level, resp, err := client.Repositories.GetPermissionLevel(ctx, "acme", "widgets", "octocat")
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(level.GetPermission()).To(Equal("write"))

	var sawInstallation bool
	for _, rec := range server.Recorder().Records() {
		if rec.AuthMode == acceptanceharness.AuthModeInstallation &&
			rec.Method == http.MethodGet &&
			strings.Contains(rec.Path, "/collaborators/") {
			sawInstallation = true
		}
	}
	Expect(sawInstallation).To(BeTrue(), "permission check must record AuthModeInstallation, got %+v", server.Recorder().Records())
}

func body_gogithubContractAcceptanceTest_mintsViaAppsAPIThenReadsContentsWithThatTokenThr_164(server *fakegithub.Server, ctx context.Context) {
	apps := newAppsClient(server)
	tok, _, err := apps.Apps.CreateInstallationToken(ctx, contractInstallationID, nil)
	Expect(err).NotTo(HaveOccurred())

	client, err := github.NewClient(
		github.WithEnterpriseURLs(server.URL(), server.URL()),
		github.WithAuthToken(tok.GetToken()),
	)
	Expect(err).NotTo(HaveOccurred())

	file, _, resp, err := client.Repositories.GetContents(ctx, "acme", "widgets", "src/main.go", &github.RepositoryContentGetOptions{Ref: "main"})
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	content, err := file.GetContent()
	Expect(err).NotTo(HaveOccurred())
	Expect(content).To(Equal("package main\n"))

	var modes []acceptanceharness.AuthMode
	for _, rec := range server.Recorder().Records() {
		modes = append(modes, rec.AuthMode)
	}
	Expect(modes).To(ContainElement(acceptanceharness.AuthModeNone))
	Expect(modes).To(ContainElement(acceptanceharness.AuthModeInstallation))
}
