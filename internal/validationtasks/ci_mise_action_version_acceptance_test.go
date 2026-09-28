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
			body_ciMiseActionVersionAcceptanceTest_failsIfAnyJdxMiseActionStepOmitsAVersionInputEqu_28(yml, toml)
		})
	})
})

func miseActionStepBodies(yml string) []string {
	lines := strings.Split(yml, "\n")
	var bodies []string
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "- uses:") || !strings.Contains(trimmed, "jdx/mise-action@") {
			continue
		}
		indent := len(lines[i]) - len(strings.TrimLeft(lines[i], " \t"))
		end := i + 1
		(&sigmiseActionStepBodiesS4{end: &end, indent: indent, lines: lines}).call()

		bodies = append(bodies, strings.Join(lines[i:end], "\n"))
		i = end - 1
	}
	return bodies
}

func miseActionVersion(step string) (string, bool) {
	for _, line := range strings.Split(step, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(trimmed, "version:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
		value = strings.Trim(value, `"'`)
		if value == "" {
			return "", false
		}
		return value, true
	}
	return "", false
}
