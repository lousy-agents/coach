package fakegithub_test

import (
	"context"
	"errors"
	"net/http"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

func body_contentsAcceptanceTest_returnsTheDecodedBytesAndMetadataAndRecordsBothT_39(server *fakegithub.Server, reader *githubingest.GitHubFileReader) {
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
}

func body_contentsAcceptanceTest_returnsGithubingestErrAuthAndTheFailureComesFrom_124(server *fakegithub.Server, reader *githubingest.GitHubFileReader) {
	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/authfail.txt"}
	_, _, err := reader.ReadFile(context.Background(), ref)

	Expect(errors.Is(err, githubingest.ErrAuth)).To(BeTrue(), "got err %v, want errors.Is(err, ErrAuth)", err)

	// ErrAuth alone is ambiguous (mint failure vs contents handler).
	// Require a recorded /contents/ request with this scenario.
	var sawContentsAuthFail bool
	for _, rec := range server.Recorder().Records() {
		if rec.Method == http.MethodGet && strings.Contains(rec.Path, "/contents/") && rec.Scenario == string(fakegithub.ScenarioAuthFail) {
			sawContentsAuthFail = true
		}
	}
	Expect(sawContentsAuthFail).To(BeTrue(), "expected a recorded GET request against a /contents/ path with scenario %q, got %+v", fakegithub.ScenarioAuthFail, server.Recorder().Records())
}

func body_contentsAcceptanceTest_returnsANonNilErrorThatMatchesNoneOfGithubingest_143(reader *githubingest.GitHubFileReader) {
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
}
