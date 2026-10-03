package githubingest_test

import (
	"context"
	"errors"
	"net/http"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

func body_acceptanceTest_targetsAConfiguredGitHubEnterpriseBaseURLInstead_73() {
	transport := &urlRecordingEnterpriseTransport{}
	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          1,
		InstallationID: 2,
		PrivateKey:     ginkgoRSAKey(),
		BaseURL:        "https://ghe.example.com/",
		Transport:      transport,
	})
	Expect(err).NotTo(HaveOccurred())

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "hello.txt"}
	_, _, err = reader.ReadFile(context.Background(), ref)
	Expect(err).NotTo(HaveOccurred())

	for _, u := range transport.seen {
		Expect(u).To(HavePrefix("https://ghe.example.com/api/v3/"))
	}
}

func body_acceptanceTest_returnsAWrappedAPIFailureErrorMatchingNoneOfTheD_195() {
	const canned = `{
				"type": "file",
				"encoding": "base64",
				"size": 5,
				"name": "bad.txt",
				"path": "dir/bad.txt",
				"sha": "badsha",
				"content": "!!!not-valid-base64!!!"
			}`
	reader := ginkgoTestReader(func(req *http.Request) *http.Response {
		return jsonResponse(req, http.StatusOK, canned)
	})

	ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/bad.txt"}
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
