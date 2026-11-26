package evalcensus_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEvalCensusAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "eval/internal/evalcensus acceptance suite")
}
