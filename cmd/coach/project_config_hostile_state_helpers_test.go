package main

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// refusedScan removes the committed policy from disk and runs a scan that must refuse it.
func refusedScan(repo, policy string, extra ...string) (stderr string) {
	args := append([]string{"codesignal", "--baseline", "--project-config", policy, "--format=json"}, extra...)
	stdout, stderrBytes, exitCode := runCoachBinary(commandPath, repo, nil, args...)
	ExpectWithOffset(1, exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderrBytes)
	ExpectWithOffset(1, stdout).To(BeEmpty())
	return string(stderrBytes)
}

func commitThenDelete(repo, name string) {
	commitFile(repo, name, singleRootPolicyJSON)
	ExpectWithOffset(1, os.Remove(filepath.Join(repo, name))).To(Succeed())
}

// commitRepoWithSubmodule is a scannable repository that records a submodule at vendor/sub, whose own
// checkout holds a coach.json.
func commitRepoWithSubmodule() string {
	repo := commitScannableGoRepo()
	addSubmodule(repo)
	return repo
}

// addSubmodule records a submodule at vendor/sub in repo, whose own checkout holds a coach.json.
func addSubmodule(repo string) { addSubmoduleAt(repo, "vendor/sub") }

// addSubmoduleAt commits a submodule at path, with its checkout there.
func addSubmoduleAt(repo, path string) {
	inner := GinkgoT().TempDir()
	gitIn(inner, "init", "-q")
	Expect(os.WriteFile(filepath.Join(inner, "coach.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
	Expect(os.MkdirAll(filepath.Join(inner, "cfg"), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(inner, "cfg", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
	gitIn(inner, "add", "-A")
	innerCommit := exec.Command("git", "commit", "-q", "-m", "inner")
	innerCommit.Dir, innerCommit.Env = inner, commitEnv
	ExpectWithOffset(1, innerCommit.Run()).To(Succeed())
	gitIn(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", inner, path)
	superCommit := exec.Command("git", "commit", "-q", "-m", "sub")
	superCommit.Dir, superCommit.Env = repo, commitEnv
	ExpectWithOffset(1, superCommit.Run()).To(Succeed())
}
