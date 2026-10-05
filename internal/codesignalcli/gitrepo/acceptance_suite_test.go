package gitrepo

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGitRepoAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/codesignalcli/gitrepo acceptance suite")
}
