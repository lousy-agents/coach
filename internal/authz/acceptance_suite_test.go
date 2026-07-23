package authz_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestAuthzAcceptance runs the Ginkgo acceptance suite for internal/authz:
// ADR-003's submit-time repository authorization check, exercised against
// internal/fakegithub over httptest -- no real GitHub credentials or network
// access.
func TestAuthzAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/authz acceptance suite")
}
