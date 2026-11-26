package evalgrade_test

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgrade"
)

const testNonce = "NONCE-7f3a91c2-e8d4"

// nodePath fails, rather than skips, when node is missing, so a run without the toolchain cannot look green.
func nodePath() string {
	path, err := exec.LookPath("node")
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "node 24 must be on PATH for the TypeScript nonce specs")
	return path
}

func tsFixture(files map[string]string) (dir string, names []string) {
	dir = GinkgoT().TempDir()
	for path, content := range files {
		ExpectWithOffset(1, os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o755)).To(Succeed())
		ExpectWithOffset(1, os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644)).To(Succeed())
		names = append(names, path)
	}
	return dir, names
}

func runFixture(files map[string]string, order ...string) evalgrade.TypeScriptRun {
	dir, names := tsFixture(files)
	if len(order) > 0 {
		names = order
	}
	run, err := evalgrade.RunTypeScriptTests(nodePath(), dir, names, testNonce)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return run
}

func statusOf(run evalgrade.TypeScriptRun, id string) []string {
	statuses := []string{}
	for _, record := range run.Records {
		if record.ID == id && record.Nonce == testNonce {
			statuses = append(statuses, record.Status)
		}
	}
	return statuses
}

const header = "import assert from \"node:assert/strict\";\nimport fs from \"node:fs\";\nimport { describe, test } from \"node:test\";\n"
