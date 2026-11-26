package evalharness_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEvalHarnessAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "evalharness acceptance suite")
}
