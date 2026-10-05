package thinproof_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/acceptanceharness/thinproof"
	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

func body_fixtureAcceptanceTest_readsTheFixtureFileByteForByteAndMetadataExactUs_19() {
	fixture := thinproof.BuildFixture()

	handler, recorder := fakegithub.Handler(&fixture)
	server := httptest.NewServer(handler)
	defer server.Close()

	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          thinproof.AppID,
		InstallationID: thinproof.InstallationID,
		PrivateKey:     acceptanceharness.GenerateRSAPrivateKeyPEM(GinkgoTB()),
		BaseURL:        server.URL,
	})
	Expect(err).NotTo(HaveOccurred())

	ref := githubingest.GitHubFileRef{
		Owner: thinproof.Owner,
		Repo:  thinproof.Repo,
		Ref:   thinproof.Ref,
		Path:  thinproof.Path,
	}
	data, meta, err := reader.ReadFile(context.Background(), ref)

	Expect(err).NotTo(HaveOccurred())
	Expect(data).To(Equal(thinproof.FileContent))
	Expect(meta).To(Equal(githubingest.FileMetadata{
		Path: thinproof.Path,
		Ref:  thinproof.Ref,
		SHA:  thinproof.FileSHA,
		Size: len(thinproof.FileContent),
	}))

	var sawTokenMint, sawContentsRead bool
	for _, rec := range recorder.Records() {
		if rec.Method == http.MethodPost && strings.Contains(rec.Path, "/access_tokens") {
			sawTokenMint = true
		}
		if rec.Method == http.MethodGet && strings.Contains(rec.Path, "/contents/") {
			sawContentsRead = true
			Expect(rec.AuthMode).To(Equal(acceptanceharness.AuthModeInstallation), "expected the Contents API read to use installation credentials, got %+v", rec)
		}
	}
	Expect(sawTokenMint).To(BeTrue(), "expected a recorded installation-token mint, got %+v", recorder.Records())
	Expect(sawContentsRead).To(BeTrue(), "expected a recorded Contents API read, got %+v", recorder.Records())
}
