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

func miseTomlMinVersion(toml string) string {
	m := miseTomlMinVersionPattern.FindStringSubmatch(toml)
	if m == nil {
		return ""
	}
	return m[1]
}

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
		for end < len(lines) {
			if strings.TrimSpace(lines[end]) == "" {
				end++
				continue
			}
			lineIndent := len(lines[end]) - len(strings.TrimLeft(lines[end], " \t"))
			if lineIndent <= indent {
				break
			}
			end++
		}
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
		})
	})
})
