package githubingest_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestGitHubIngestAcceptance runs the Ginkgo acceptance suite documenting
// Story 5 of GitHub issue #1 ("Feature: Semantic Analysis Module") for
// pkg/githubingest: optional, offline GitHub App file ingestion exercised
// entirely through the public GitHubFileReader API and a fake
// http.RoundTripper. It complements, rather than replaces, the existing
// black-box *_test.go coverage in this package -- see the acceptance-
// coverage matrix in the PR description for which AC each style of test
// carries. It reuses the offline test fixtures (generateTestRSAPrivateKeyPEM,
// fakeGitHubTransport, jsonResponse, ...) defined in testhelpers_test.go: no
// network access or real credentials are used anywhere in this package.
func TestGitHubIngestAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "pkg/githubingest acceptance suite")
}
