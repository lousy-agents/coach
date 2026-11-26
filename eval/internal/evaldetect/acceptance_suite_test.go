package evaldetect_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEvalDetectAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "eval/internal/evaldetect acceptance suite")
}
