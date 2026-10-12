package revisionfs

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRevisionFSAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/codesignalcli/revisionfs acceptance suite")
}
