package validationtasks

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestValidationTasksAcceptance runs the Ginkgo acceptance suite for the
// repository's mise validation-task contract. The specs read mise.toml from
// the repository root and assert task composition only, so they are offline
// and execute no build or test command.
func TestValidationTasksAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/validationtasks acceptance suite")
}
