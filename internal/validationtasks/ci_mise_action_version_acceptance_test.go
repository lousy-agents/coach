package validationtasks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var miseTomlMinVersionPattern = regexp.MustCompile(`(?m)^min_version\s*=\s*"([^"]+)"`)

var _ = Describe("CI mise-action version pin", func() {
	var yml, toml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
		Expect(err).NotTo(HaveOccurred())
		yml = string(raw)
		raw, err = os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
	})

	When("the acceptance suite runs", func() {
		It("fails if any jdx/mise-action step omits a version input equal to mise.toml min_version", func() {
			expectEveryMiseActionPinnedToMinVersion(yml, toml)
		})
	})
})

func expectEveryMiseActionPinnedToMinVersion(yml string, toml string) {
	minVersion := miseTomlMinVersion(toml)
	Expect(minVersion).NotTo(BeEmpty(),
		"mise.toml must declare min_version so the CI pin has a source of truth")

	steps := miseActionStepBodies(yml)
	Expect(steps).NotTo(BeEmpty(),
		"ci.yml must contain jdx/mise-action steps or this spec cannot catch an unpinned install")

	var unpinned []string
	for _, step := range steps {
		version, ok := miseActionVersion(step)
		if !ok || version != minVersion {
			unpinned = append(unpinned, step)
		}
	}
	Expect(unpinned).To(BeEmpty(),
		"%d jdx/mise-action step(s) omit a version input equal to mise.toml min_version %q:\n%s",
		len(unpinned), minVersion, strings.Join(unpinned, "\n\n"))
}
