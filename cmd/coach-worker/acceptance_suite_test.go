package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestCoachWorkerAcceptance runs the Ginkgo acceptance suite for
// cmd/coach-worker (Task 3 / GitHub issue #104).
func TestCoachWorkerAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "cmd/coach-worker acceptance suite")
}
