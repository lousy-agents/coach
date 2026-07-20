package fakegithub_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestFakeGitHubAcceptance runs the Ginkgo acceptance suite documenting
// GitHub issue #77 ("Task 0.2: Implement the Coach-owned fake GitHub
// service", epic #73 "Feature Zero: Offline Acceptance Foundation") for
// internal/fakegithub: OAuth authorization-code/token/"/user", GitHub App
// installation-token minting, repo-to-installation resolution, effective
// permissions, and repository content reads, all exercised against a real
// in-process HTTP server (fakegithub.Server) driven by fixture data. The
// pkg/githubingest-facing coverage in contents_acceptance_test.go exercises
// that package exclusively through its exported API, never its internals.
func TestFakeGitHubAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/fakegithub acceptance suite")
}
