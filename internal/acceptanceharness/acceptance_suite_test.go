package acceptanceharness_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestAcceptanceHarnessAcceptance is the single Ginkgo suite bootstrap for
// internal/acceptanceharness, covering all three Feature-Zero Task 0.1
// pieces of GitHub issue #76 ("Task 0.1: Define the acceptance harness
// contract and offline guard"), part of epic #73 ("Feature Zero: Offline
// Acceptance Foundation"): the ambient-credential guard and no-egress
// transport (CredentialGuardResult, ScanEnviron/ScanProcessEnv/
// ScrubProcessEnv, GuardedTransport), the controlled-clock seam
// (Clock/RealClock/FakeClock), and the generated test-credential helpers.
// Each piece's specs are registered by its own *_acceptance_test.go file in
// this package, but Ginkgo requires exactly one RunSpecs call per package,
// so this file is the sole bootstrap. Entirely offline: no real network
// access or real credentials are used anywhere in this package.
func TestAcceptanceHarnessAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/acceptanceharness acceptance suite")
}
