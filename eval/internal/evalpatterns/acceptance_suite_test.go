package evalpatterns_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEvalPatternsAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "eval/internal/evalpatterns acceptance suite")
}
