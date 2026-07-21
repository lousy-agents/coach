package fakegithub_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestFakeGitHubAcceptance runs the fakegithub offline acceptance suite
// (raw HTTP, go-github contract, githubingest public API, cross-cutting).
func TestFakeGitHubAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/fakegithub acceptance suite")
}
