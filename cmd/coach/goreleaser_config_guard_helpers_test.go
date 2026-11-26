package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

func goreleaserConfiguration() string {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(), ".goreleaser.yaml"))
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}

func goreleaserConfigurationViolations(content string) []string {
	var violations []string
	if strings.Contains(content, evalOverrideBuildTag) {
		violations = append(violations, "names the tag "+evalOverrideBuildTag)
	}
	for _, line := range strings.Split(content, "\n") {
		if key, declared := declaredBuildOption(line); declared {
			violations = append(violations, "declares "+key)
		}
	}
	return violations
}

func declaredBuildOption(line string) (key string, declared bool) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
	for _, option := range []string{"tags", "flags", "ldflags"} {
		if strings.HasPrefix(trimmed, option+":") {
			return option, true
		}
	}
	return "", false
}
