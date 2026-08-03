package claudehooks

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestClaudeHooksAcceptance runs the Ginkgo acceptance suite for the repository's
// .claude/hooks shell hooks. The specs drive each hook as a subprocess against
// fake tool binaries, so they are offline and touch nothing outside t.TempDir().
// It complements the existing stdlib black-box coverage in this package rather
// than replacing it.
func TestClaudeHooksAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/claudehooks acceptance suite")
}
