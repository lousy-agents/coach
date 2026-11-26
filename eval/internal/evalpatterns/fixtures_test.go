package evalpatterns_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// runGit runs git in dir with the fixture author fixed, so commits do not
// depend on the host's identity.
func runGit(dir string, args ...string) string {
	GinkgoHelper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git %s: %s", strings.Join(args, " "), output)
	return strings.TrimSpace(string(output))
}

func writeFile(path, content string) {
	GinkgoHelper()
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	Expect(os.WriteFile(path, []byte(content), 0o644)).To(Succeed())
}

// referenceRepository makes a local Go repository with one commit and
// returns its path and the commit's SHA.
func referenceRepository() (string, string) {
	GinkgoHelper()
	dir := filepath.Join(GinkgoT().TempDir(), "reference")
	Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
	runGit(dir, "init", "--quiet", "--initial-branch=main")
	writeFile(filepath.Join(dir, "go.mod"), "module example.invalid/reference\n\ngo 1.22\n")
	writeFile(filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	runGit(dir, "add", "-A")
	runGit(dir, "commit", "--quiet", "-m", "reference")
	return dir, runGit(dir, "rev-parse", "HEAD")
}

func writeManifest(content any) string {
	GinkgoHelper()
	raw, err := json.Marshal(content)
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(GinkgoT().TempDir(), "references.json")
	Expect(os.WriteFile(path, raw, 0o644)).To(Succeed())
	return path
}

const sampleSHA = "0123456789abcdef0123456789abcdef01234567"

// validProfile is a profile that satisfies every rule of the schema; each
// spec changes one thing in a copy of it.
func validProfile() map[string]any {
	return map[string]any{
		"schema_version":    "1",
		"identity":          strings.Repeat("ab", 32),
		"label":             "stand-in",
		"sha":               sampleSHA,
		"licence":           "MIT",
		"extractor_version": "0.2.0",
		"roles": map[string]any{
			"api":   map[string]any{"packages": map[string]any{"1": 1}},
			"store": map[string]any{"packages": map[string]any{"2": 1}},
		},
		"dimensions": map[string]any{
			"layers":    map[string]any{"3": 1},
			"entry":     map[string]any{"http_handler": 0, "method": 4, "function": 2},
			"depth":     map[string]any{"2": 3},
			"trap":      nil,
			"field":     nil,
			"direction": nil,
		},
		"shapes": map[string]any{
			"nesting":        map[string]any{"1": 5, "4": 1},
			"cognitive":      map[string]any{"0": 2},
			"branch_density": map[string]any{"3": 1},
			"constructor":    map[string]any{"assigns_only": 1, "does_work": 0},
		},
		"not_covered": []any{map[string]any{"shape": "layers", "reason": "no_total_order", "count": 1}},
	}
}

func profileJSON(profile map[string]any) []byte {
	GinkgoHelper()
	raw, err := json.Marshal(profile)
	Expect(err).NotTo(HaveOccurred())
	return raw
}
