package validationtasks

import (
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const rootSkipCovers = "mise run acceptance-ci-env"

var _ = Describe("a test that cannot build its state as root (#733 AC6)", func() {
	It("#733 AC6: names `mise run acceptance-ci-env` in its root skip reason, because that task covers the non-root case", func() {
		skips := rootSkipReasons(filepath.Join("..", ".."))
		Expect(skips).NotTo(BeEmpty(), "sanity: the repository has root skips")

		Expect(withoutCoverNote(skips)).To(BeEmpty(), "each root skip must say that %q runs it as non-root", rootSkipCovers)
	})
})

func withoutCoverNote(skips []string) []string {
	var missing []string
	for _, skip := range skips {
		if !strings.Contains(skip, rootSkipCovers) {
			missing = append(missing, skip)
		}
	}
	return missing
}
