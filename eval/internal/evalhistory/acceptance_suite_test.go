package evalhistory_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEvalHistoryAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "eval/internal/evalhistory acceptance suite")
}
