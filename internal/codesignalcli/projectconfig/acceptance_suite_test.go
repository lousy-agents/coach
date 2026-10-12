package projectconfig

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestProjectConfigAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/codesignalcli/projectconfig acceptance suite")
}
