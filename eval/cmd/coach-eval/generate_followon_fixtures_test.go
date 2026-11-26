package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	. "github.com/onsi/gomega"
)

var failedPackage = regexp.MustCompile(`(?m)^FAIL[ \t]+(\S+)`)

func failingPackages(output generateOutcome) []string {
	var packages []string
	for _, match := range failedPackage.FindAllStringSubmatch(output.stdout+output.stderr, -1) {
		if strings.Contains(match[1], "/") {
			packages = append(packages, match[1])
		}
	}
	sort.Strings(packages)
	return packages
}

func manifestOf(program generatedProgram) map[string]any {
	content, err := os.ReadFile(filepath.Join(filepath.Dir(program.Path), "manifest.json"))
	Expect(err).NotTo(HaveOccurred())
	manifest := map[string]any{}
	Expect(json.Unmarshal(content, &manifest)).To(Succeed())
	return manifest
}

func followOnField(manifest map[string]any) string {
	template := manifest["template"].(map[string]any)
	followOn, present := template["follow_on"].(map[string]any)
	Expect(present).To(BeTrue(), "the manifest's template names the follow-on")
	return followOn["field"].(string)
}

func followOnFirstField(manifest map[string]any) string {
	template := manifest["template"].(map[string]any)
	return template["follow_on"].(map[string]any)["first_field"].(string)
}
