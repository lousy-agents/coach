package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestCoachAPIAcceptance runs the Ginkgo acceptance suite for cmd/coach-api
// (Task 2 / GitHub issue #103): proves buildHandler composes internal/authn
// and internal/coachapi into one HTTP surface, since each package's own
// suite only exercises them in isolation.
func TestCoachAPIAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "cmd/coach-api acceptance suite")
}
