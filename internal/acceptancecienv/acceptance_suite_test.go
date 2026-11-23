package acceptancecienv

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestAcceptanceCIEnvAcceptance runs the specs for `mise run
// acceptance-ci-env`, which runs Go tests under CI's conditions: a non-root
// user, an empty HOME, no user-level git configuration, and CI's pinned git.
// Unlike internal/validationtasks, these specs build and run the task.
func TestAcceptanceCIEnvAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/acceptancecienv acceptance suite")
}
