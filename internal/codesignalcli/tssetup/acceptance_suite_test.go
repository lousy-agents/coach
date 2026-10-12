package tssetup

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTSSetupAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/codesignalcli/tssetup acceptance suite")
}
