package codesignal_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCodeSignalAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CodeSignal acceptance suite")
}
