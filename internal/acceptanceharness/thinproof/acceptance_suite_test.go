package thinproof_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestThinProofAcceptance runs the internal/acceptanceharness/thinproof
// offline acceptance suite: proof that the shared fixture served by
// fakegithub.Handler is readable end-to-end through pkg/githubingest's
// public GitHubFileReader API, entirely offline.
func TestThinProofAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/acceptanceharness/thinproof acceptance suite")
}
