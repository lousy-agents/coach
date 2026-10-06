package main

import (
	"encoding/json"
	"os/exec"

	. "github.com/onsi/gomega"
)

const singleRootPolicyJSON = `{"schema_version":"1","roots":["."]}` + "\n"

// runCoachCheckProjectEnv runs `coach codesignal [args...]` in repo with a
// caller-controlled PATH (plus the host's HOME, so git can find its global
// config), returning raw stdout/stderr without assuming success. Unlike
// runCoachSuggest, it does not inherit the test process's ambient
// environment: checkNodeReadiness shells out to whatever `node` is first on
// the child's PATH, so a deterministic node-dependent spec must control that
// PATH.
func runCoachCheckProjectEnv(workingDir, path string, args ...string) (stdout, stderr []byte, exitCode int) {
	return runCoachBinary(commandPath, workingDir, stubToolchainEnv(path), append([]string{"codesignal"}, args...)...)
}

// gitStatusPorcelain reports repo's worktree status, so a spec can prove no
// file inside it was created, modified, or removed between two points in
// time: a failed install must never leave Coach itself having touched the
// repository.
func gitStatusPorcelain(repo string) string {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repo
	output, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())
	return string(output)
}

// noSupportedCompilerRepo commits a minimal TypeScript-shaped, policy-ready
// repository with no installed or declared compiler at all, so
// checks.compiler fails with typescript_compiler_missing and neither mise
// scope has anything configured -- the fixture every prepare_compiler mise
// setup spec starts from.
func noSupportedCompilerRepo() string {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
	return repo
}

func checkProjectBothFormats(repo, path string, extraArgs ...string) (readinessResultDoc, string) {
	base := append([]string{"--baseline", "--check-project", "--project-language", "typescript"}, extraArgs...)

	jsonArgs := append(append([]string{}, base...), "--format", "json")
	jsonStdout, jsonStderr, jsonExit := runCoachCheckProjectEnv(repo, path, jsonArgs...)
	ExpectWithOffset(1, jsonExit).To(Equal(0), "stderr: %s", jsonStderr)
	var doc readinessResultDoc
	ExpectWithOffset(1, json.Unmarshal(jsonStdout, &doc)).To(Succeed(), "stdout: %s", jsonStdout)

	textArgs := append([]string{}, base...)
	textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, textArgs...)
	ExpectWithOffset(1, textExit).To(Equal(0), "stderr: %s", textStderr)

	return doc, string(textStdout)
}
