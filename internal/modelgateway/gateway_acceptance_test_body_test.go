package modelgateway_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

func body_gatewayAcceptanceTest_keepsProductionIdentifiersFreeOfProviderSpecific_129() {
	dir, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())

	entries, err := os.ReadDir(dir)
	Expect(err).NotTo(HaveOccurred())

	forbidden := []string{"llamacpp", "llama.cpp", "sglang"}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		lowerName := strings.ToLower(name)
		for _, f := range forbidden {
			Expect(lowerName).NotTo(ContainSubstring(f), "production filename %s", name)
		}
		raw, readErr := os.ReadFile(filepath.Join(dir, name))
		Expect(readErr).NotTo(HaveOccurred())
		lower := strings.ToLower(string(raw))
		for _, f := range forbidden {
			Expect(lower).NotTo(ContainSubstring(f), "production file %s", name)
		}
	}
}

func body_gatewayAcceptanceTest_isTheOnlyNonTestPackageThatOwnsTheChatCompletion_155() {
	root := findModuleRoot()
	Expect(root).NotTo(BeEmpty())

	var offenders []string
	err := filepath.WalkDir(root, (&sigbodygatewayAcceptanceTestisTheOnlyNonTestPackageThatOwnsT{offenders: &offenders, root: root}).call)
	Expect(err).NotTo(HaveOccurred())
	Expect(offenders).To(BeEmpty(), "chat-completions path must stay inside internal/modelgateway: %v", offenders)
}
