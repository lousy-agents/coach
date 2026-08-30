package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type readinessCheckDoc struct {
	State           string `json:"state"`
	Code            string `json:"code"`
	Version         string `json:"version"`
	ExpectedVersion string `json:"expected_version"`
	FoundVersion    string `json:"found_version"`
}

type readinessResultDoc struct {
	SchemaVersion string `json:"schema_version"`
	Status        string `json:"status"`
	Language      string `json:"language"`
	Revision      string `json:"revision"`
	DirtyWorktree struct {
		RelevantChanges bool     `json:"relevant_changes"`
		Paths           []string `json:"paths"`
	} `json:"dirty_worktree"`
	Checks struct {
		ProjectShape readinessCheckDoc `json:"project_shape"`
		Policy       readinessCheckDoc `json:"policy"`
		Node         readinessCheckDoc `json:"node"`
		Compiler     readinessCheckDoc `json:"compiler"`
	} `json:"checks"`
	Gaps []struct {
		Code string `json:"code"`
	} `json:"gaps"`
	Warnings []struct {
		Code        string `json:"code"`
		FoundMajor  int    `json:"found_major"`
		TestedMajor int    `json:"tested_major"`
		FloorMajor  int    `json:"floor_major"`
	} `json:"warnings"`
	NextActions []struct {
		Kind string `json:"kind"`
	} `json:"next_actions"`
}

func gapCodes(doc readinessResultDoc) []string {
	codes := make([]string, len(doc.Gaps))
	for i, g := range doc.Gaps {
		codes[i] = g.Code
	}
	return codes
}

func nextActionKinds(doc readinessResultDoc) []string {
	kinds := make([]string, len(doc.NextActions))
	for i, a := range doc.NextActions {
		kinds[i] = a.Kind
	}
	return kinds
}

// writeStubNodeScript writes an executable `node` script into a fresh temp
// directory that always prints version regardless of its arguments, and
// returns that directory. checkNodeReadiness's detectHostNodeMajor shells
// out to whatever `node` is first on the child process's PATH, so a spec
// that wants a specific, host-independent Node major must control PATH with
// a stub rather than depend on whatever Node happens to be installed.
func writeStubNodeScript(version string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubnode-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := fmt.Sprintf("#!/bin/sh\necho %s\n", version)
	Expect(os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathWithStubNode returns a PATH whose first entry is a stub `node`
// reporting version, with every directory containing a real node/npm
// executable removed so the stub is the only "node" the child process can
// resolve.
func pathWithStubNode(version string) string {
	return writeStubNodeScript(version) + string(os.PathListSeparator) + pathExcludingExecutables("node", "npm")
}

// pathWithoutNode returns a PATH with every node/npm directory removed, so
// checkNodeReadiness deterministically reports node_missing regardless of
// the host's actual Node installation.
func pathWithoutNode() string {
	return pathExcludingExecutables("node", "npm")
}

// requireStubNodeVersion is the belt-and-suspenders probe mirroring
// node_absent_acceptance_test.go's pattern: it proves path's stub node is
// genuinely the one that would be resolved and reports exactly version, so
// a deterministic result below cannot be a false green caused by some other
// node still being reachable.
func requireStubNodeVersion(path, wantVersion string) {
	probe := exec.Command("sh", "-c", "node --version")
	probe.Env = []string{"PATH=" + path}
	output, err := probe.Output()
	Expect(err).NotTo(HaveOccurred(), "expected the stub node to be reachable on %q", path)
	Expect(strings.TrimSpace(string(output))).To(Equal(wantVersion))
}

// requireNodeUnreachable mirrors node_absent_acceptance_test.go's probe: it
// proves neither node nor npm resolves on path, so a deterministic
// node_missing result below cannot be a false green.
func requireNodeUnreachable(path string) {
	probe := exec.Command("sh", "-c", "command -v node || command -v npm")
	probe.Env = []string{"PATH=" + path}
	Expect(probe.Run()).To(HaveOccurred(), "expected neither node nor npm to be found on %q", path)
}

// runCoachCheckProjectEnv runs `coach codesignal [args...]` in repo with a
// caller-controlled PATH (plus the host's HOME, so git can find its global
// config), returning raw stdout/stderr without assuming success. Unlike
// runCoachSuggest, it does not inherit the test process's ambient
// environment: checkNodeReadiness shells out to whatever `node` is first on
// the child's PATH, so a deterministic node-dependent spec must control that
// PATH.
func runCoachCheckProjectEnv(workingDir, path string, args ...string) (stdout, stderr []byte, exitCode int) {
	command := exec.Command(commandPath, append([]string{"codesignal"}, args...)...)
	command.Dir = workingDir
	command.Env = []string{"PATH=" + path, "HOME=" + os.Getenv("HOME")}
	var outBuf, errBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &errBuf

	err := command.Run()
	if err == nil {
		return outBuf.Bytes(), errBuf.Bytes(), 0
	}

	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
}

// corruptCommittedBlob deletes path's loose object file after it has been
// committed, leaving the commit/tree objects (and thus revision resolution)
// intact while making the blob itself unreadable.
func corruptCommittedBlob(repo, path string) {
	revCmd := exec.Command("git", "rev-parse", "HEAD:"+path)
	revCmd.Dir = repo
	output, err := revCmd.Output()
	Expect(err).NotTo(HaveOccurred())
	blobSHA := strings.TrimSpace(string(output))
	Expect(blobSHA).To(HaveLen(40))

	objectPath := filepath.Join(repo, ".git", "objects", blobSHA[:2], blobSHA[2:])
	_, statErr := os.Stat(objectPath)
	Expect(statErr).NotTo(HaveOccurred(), "expected a loose object at %s -- was the fixture repo gc'd?", objectPath)
	Expect(os.Remove(objectPath)).To(Succeed())
}

// corruptCommittedTree deletes dirPath's own subtree loose object after it
// has been committed, leaving the parent tree/commit objects (and thus
// revision resolution) intact while making a path underneath dirPath
// unresolvable. Unlike corruptCommittedBlob (which corrupts the leaf blob
// itself), this exercises a git-plumbing call that only walks tree objects
// without ever opening blob content.
func corruptCommittedTree(repo, dirPath string) {
	revCmd := exec.Command("git", "rev-parse", "HEAD:"+dirPath)
	revCmd.Dir = repo
	output, err := revCmd.Output()
	Expect(err).NotTo(HaveOccurred())
	treeSHA := strings.TrimSpace(string(output))
	Expect(treeSHA).To(HaveLen(40))

	objectPath := filepath.Join(repo, ".git", "objects", treeSHA[:2], treeSHA[2:])
	_, statErr := os.Stat(objectPath)
	Expect(statErr).NotTo(HaveOccurred(), "expected a loose object at %s -- was the fixture repo gc'd?", objectPath)
	Expect(os.Remove(objectPath)).To(Succeed())
}

// writeHangingNodeScript writes an executable `node` script that never
// terminates on its own, returning the directory containing it. `exec sleep
// N` replaces the shell's own process image, so killing the script's PID
// (as a context deadline does) kills the sleep directly instead of leaving
// it as an orphaned child.
func writeHangingNodeScript() string {
	dir, err := os.MkdirTemp("", "coach-acceptance-hangnode-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := "#!/bin/sh\nexec sleep 30\n"
	Expect(os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathWithHangingNode returns a PATH whose first entry is a stub `node`
// that hangs indefinitely on `--version`, with every directory containing a
// real node/npm executable removed so the stub is the only "node" the
// child process can resolve.
func pathWithHangingNode() string {
	return writeHangingNodeScript() + string(os.PathListSeparator) + pathExcludingExecutables("node", "npm")
}

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript", func() {
	When("HEAD has a TypeScript-shaped project (package.json) but no project.json policy, and Node is a supported major", func() {
		It("exits 0, reports status needs_policy, policy fail/policy_missing, compiler not_checked, and an author_policy next action (JSON)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")
			requireStubNodeVersion(path, "v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.SchemaVersion).To(Equal("1"))
			Expect(doc.Language).To(Equal("typescript"))
			Expect(doc.Status).To(Equal("needs_policy"))
			Expect(doc.Checks.Policy.State).To(Equal("fail"))
			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
			Expect(doc.Checks.Compiler.State).To(Equal("not_checked"))
			Expect(doc.Checks.Node.State).To(Equal("pass"), "the stubbed, supported Node major must report pass deterministically")
			Expect(gapCodes(doc)).To(ContainElement("policy_missing"))
			Expect(nextActionKinds(doc)).To(ContainElement("author_policy"))
		})

		It("renders the same status, gaps, and next actions in the default text format", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("status: needs_policy"))
			Expect(text).To(ContainSubstring("policy: fail (policy_missing)"))
			Expect(text).To(ContainSubstring("compiler: not_checked"))
			Expect(text).To(ContainSubstring("author_policy"))
		})
	})

	When("HEAD has a committed, valid project.json policy plus tsconfig.json and package.json, and Node is a supported major", func() {
		It("reports the policy check as pass and, with every other check clean, an overall status of ready", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("pass"))
			Expect(doc.Checks.Policy.Code).To(BeEmpty())
			Expect(gapCodes(doc)).To(BeEmpty())
			Expect(doc.Status).To(Equal("ready"))
		})
	})

	When("coach is invoked from a committed subdirectory of the repository rather than the repository root", func() {
		It("resolves package.json and the --project-config policy against the repository root, reporting the same ready verdict as from the root", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "sub/marker.txt", "committed subdirectory\n")

			path := pathWithStubNode("v24.9.9")

			rootStdout, rootStderr, rootExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(rootExit).To(Equal(0), "stderr: %s", rootStderr)
			var rootDoc readinessResultDoc
			Expect(json.Unmarshal(rootStdout, &rootDoc)).To(Succeed(), "stdout: %s", rootStdout)

			subDir := filepath.Join(repo, "sub")
			subStdout, subStderr, subExit := runCoachCheckProjectEnv(subDir, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(subExit).To(Equal(0), "stderr: %s", subStderr)
			var subDoc readinessResultDoc
			Expect(json.Unmarshal(subStdout, &subDoc)).To(Succeed(), "stdout: %s", subStdout)

			Expect(subDoc.Checks.ProjectShape).To(Equal(rootDoc.Checks.ProjectShape))
			Expect(subDoc.Checks.Policy).To(Equal(rootDoc.Checks.Policy))
			Expect(subDoc.Checks.ProjectShape.State).To(Equal("pass"))
			Expect(subDoc.Checks.Policy.State).To(Equal("pass"))
			Expect(gapCodes(subDoc)).To(BeEmpty())
			Expect(subDoc.Status).To(Equal("ready"))
		})
	})

	When("a relevant file is modified in the worktree without being committed, while HEAD already has a real (policy) gap", func() {
		It("reports dirty_worktree.relevant_changes true with the path, as a warning that never overrides the HEAD-derived needs_policy status", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			cleanStdout, cleanStderr, cleanExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(cleanExit).To(Equal(0), "stderr: %s", cleanStderr)
			var cleanDoc readinessResultDoc
			Expect(json.Unmarshal(cleanStdout, &cleanDoc)).To(Succeed())
			Expect(cleanDoc.Status).To(Equal("needs_policy"))
			Expect(cleanDoc.DirtyWorktree.RelevantChanges).To(BeFalse())

			Expect(os.WriteFile(filepath.Join(repo, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true}}`+"\n"), 0o644)).To(Succeed())

			dirtyStdout, dirtyStderr, dirtyExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(dirtyExit).To(Equal(0), "stderr: %s", dirtyStderr)
			var dirtyDoc readinessResultDoc
			Expect(json.Unmarshal(dirtyStdout, &dirtyDoc)).To(Succeed())

			Expect(dirtyDoc.DirtyWorktree.RelevantChanges).To(BeTrue())
			Expect(dirtyDoc.DirtyWorktree.Paths).To(ContainElement("tsconfig.json"))
			Expect(gapCodes(dirtyDoc)).To(Equal(gapCodes(cleanDoc)), "a dirty worktree must never itself become a gap")
			Expect(dirtyDoc.Status).To(Equal("needs_policy"), "the uncommitted tsconfig.json edit must never override the HEAD-derived needs_policy status, proving the worktree change never becomes analysis input")
			Expect(dirtyDoc.Checks.Policy).To(Equal(cleanDoc.Checks.Policy), "worktree content must never leak into a snapshot check's result")
		})
	})

	When("a relevant file is modified in the worktree without being committed, while HEAD is otherwise fully ready (no gaps)", func() {
		It("elevates status to ready_with_limits, since that limit class is itself part of the frozen precedence", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			cleanStdout, cleanStderr, cleanExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(cleanExit).To(Equal(0), "stderr: %s", cleanStderr)
			var cleanDoc readinessResultDoc
			Expect(json.Unmarshal(cleanStdout, &cleanDoc)).To(Succeed())
			Expect(gapCodes(cleanDoc)).To(BeEmpty(), "sanity check: this fixture must have no real gaps so the limit class alone decides status")
			Expect(cleanDoc.Status).To(Equal("ready"))

			Expect(os.WriteFile(filepath.Join(repo, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true}}`+"\n"), 0o644)).To(Succeed())

			dirtyStdout, dirtyStderr, dirtyExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(dirtyExit).To(Equal(0), "stderr: %s", dirtyStderr)
			var dirtyDoc readinessResultDoc
			Expect(json.Unmarshal(dirtyStdout, &dirtyDoc)).To(Succeed())

			Expect(dirtyDoc.Status).To(Equal("ready_with_limits"))
			Expect(gapCodes(dirtyDoc)).To(BeEmpty(), "the dirty worktree must never itself become a gap")
		})

		It("renders the exact warning wording in the default text format, listing the uncommitted path", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			Expect(os.WriteFile(filepath.Join(repo, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true}}`+"\n"), 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("Warning: uncommitted or untracked changes exist under paths relevant to this result. "))
			Expect(text).To(ContainSubstring("These paths are not part of the analyzed revision and had no effect on the checks above:"))
			Expect(text).To(ContainSubstring("tsconfig.json"))
		})
	})

	When("HEAD has no committed package.json at all", func() {
		It("exits 0 and reports the top-precedence status outside_support with an unsupported_repository_shape gap and confirm_repository_shape next action", func() {
			repo := newTempGitRepo()
			commitFile(repo, "README.md", "no package.json here\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Status).To(Equal("outside_support"))
			Expect(doc.Checks.ProjectShape.State).To(Equal("fail"))
			Expect(doc.Checks.ProjectShape.Code).To(Equal("unsupported_repository_shape"))
			Expect(gapCodes(doc)).To(ContainElement("unsupported_repository_shape"))
			Expect(nextActionKinds(doc)).To(ContainElement("confirm_repository_shape"))
		})
	})

	When("HEAD has a committed, valid project.json policy declaring a non-root root, with package.json only under that root", func() {
		It("reports project_shape pass and a status other than outside_support, since the declared root names exactly where the TypeScript project lives", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["sub"]}`+"\n")
			commitFile(repo, "sub/package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "sub/tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("pass"))
			Expect(doc.Checks.ProjectShape.State).To(Equal("pass"))
			Expect(doc.Checks.ProjectShape.Code).To(BeEmpty())
			Expect(doc.Status).NotTo(Equal("outside_support"))
			Expect(gapCodes(doc)).NotTo(ContainElement("unsupported_repository_shape"))
		})
	})

	When("HEAD has a committed, valid project.json policy declaring a non-root root, with no package.json under that root or the repository root", func() {
		It("still reports project_shape fail/unsupported_repository_shape and status outside_support, proving a passing policy alone is never trusted without a real package.json hit", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["sub"]}`+"\n")
			commitFile(repo, "sub/tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("pass"))
			Expect(doc.Checks.ProjectShape.State).To(Equal("fail"))
			Expect(doc.Checks.ProjectShape.Code).To(Equal("unsupported_repository_shape"))
			Expect(doc.Status).To(Equal("outside_support"))
			Expect(gapCodes(doc)).To(ContainElement("unsupported_repository_shape"))
		})
	})

	When("HEAD has an invalid project.json policy (policy_invalid) declaring a non-root root, with package.json present under that root", func() {
		It("still reports project_shape fail/unsupported_repository_shape and status outside_support end-to-end, when checkPolicy's invalid-policy path also returns nil roots", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"2","roots":["sub"]}`+"\n")
			commitFile(repo, "sub/package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "sub/tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("fail"))
			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(doc.Checks.ProjectShape.State).To(Equal("fail"))
			Expect(doc.Checks.ProjectShape.Code).To(Equal("unsupported_repository_shape"))
			Expect(doc.Status).To(Equal("outside_support"))
			Expect(gapCodes(doc)).To(ContainElement("unsupported_repository_shape"))
		})
	})

	When("HEAD has a committed project.json policy whose roots list exceeds the roots-count budget", func() {
		It("reports policy fail/policy_invalid and exits 0 quickly, rather than fanning out into a git child process per declared root", func() {
			repo := newTempGitRepo()

			// internal/codesignalcli.maxProjectConfigRoots is 256; one more
			// than that must be rejected before checkProjectShape ever walks
			// the roots list, so this repository is deliberately configured
			// so the *old*, unbounded behavior would instead fan out into a
			// git child-process probe per root (none of which exist) and
			// report unsupported_repository_shape, not policy_invalid --
			// pinning that the roots-count budget, not merely a slow scan
			// that still eventually finds nothing, is what stops this case.
			const oversizedRootsCount = 257
			var roots strings.Builder
			roots.WriteString(`{"schema_version":"1","roots":[`)
			for i := 0; i < oversizedRootsCount; i++ {
				if i > 0 {
					roots.WriteByte(',')
				}
				fmt.Fprintf(&roots, `"root%d"`, i)
			}
			roots.WriteString(`]}` + "\n")
			commitFile(repo, "project.json", roots.String())

			path := pathWithStubNode("v24.9.9")

			started := time.Now()
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			elapsed := time.Since(started)
			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)
			Expect(elapsed).To(BeNumerically("<", 10*time.Second), "an oversized roots list must be rejected before fanning out into a git child process per root; elapsed=%s", elapsed)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("fail"))
			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(doc.Checks.ProjectShape.State).To(Equal("fail"))
			Expect(doc.Checks.ProjectShape.Code).To(Equal("unsupported_repository_shape"), "with the policy rejected, roots must never be consulted for project_shape, leaving only the root-level package.json check")
			Expect(doc.Status).To(Equal("outside_support"), "unsupported_repository_shape outranks the simultaneous policy_invalid gap")
			Expect(gapCodes(doc)).To(ConsistOf("policy_invalid", "unsupported_repository_shape"))
		})
	})

	When("Node cannot be found on the child process's PATH at all", func() {
		It("reports the node check as fail/node_missing deterministically", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithoutNode()
			requireNodeUnreachable(path)

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_missing"))
			Expect(doc.Status).To(Equal("needs_prerequisite"))
		})
	})

	When("the resolvable Node's major version is below the minimum supported major", func() {
		It("reports the node check as fail/node_below_minimum with the found version, deterministically", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v22.10.0")
			requireStubNodeVersion(path, "v22.10.0")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_below_minimum"))
			Expect(doc.Checks.Node.FoundVersion).To(Equal("v22.10.0"))
			Expect(doc.Status).To(Equal("needs_prerequisite"))
		})
	})

	When("the resolvable Node's major version is at or above the minimum but differs from the tested major", func() {
		It("reports the node check as pass/node_untested with the found version, elevating status to ready_with_limits rather than a gap", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v26.0.0")
			requireStubNodeVersion(path, "v26.0.0")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("pass"))
			Expect(doc.Checks.Node.Code).To(Equal("node_untested"))
			Expect(doc.Checks.Node.Version).To(Equal("v26.0.0"))
			Expect(gapCodes(doc)).To(BeEmpty(), "an above-floor, untested Node major is a warning, never a gap")
			Expect(doc.Status).To(Equal("ready_with_limits"))
			Expect(doc.Warnings).To(HaveLen(1), "the frozen warnings entry must be emitted for node_untested")
			Expect(doc.Warnings[0].Code).To(Equal("node_untested"))
			Expect(doc.Warnings[0].FoundMajor).To(Equal(26))
			Expect(doc.Warnings[0].TestedMajor).To(Equal(24))
			Expect(doc.Warnings[0].FloorMajor).To(Equal(24))
		})
	})

	When("Node resolves to exactly this build's tested major", func() {
		It("reports the node check as pass with no code, warning, or gap", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")
			requireStubNodeVersion(path, "v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("pass"))
			Expect(doc.Checks.Node.Code).To(BeEmpty())
			Expect(doc.Checks.Node.Version).To(Equal("v24.9.9"))
			Expect(doc.Status).To(Equal("ready"))
			Expect(doc.Warnings).To(BeEmpty(), "the tested major must never emit a node_untested warning")
		})
	})

	When("two independently discoverable gaps exist at once (policy_missing and node_below_minimum)", func() {
		It("reports both gaps in gaps[], with status reflecting only the higher-precedence one", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			// Deliberately no committed project.json, so policy_missing
			// (rank 2) is real at the same time as the stubbed
			// node_below_minimum (rank 3) below.

			path := pathWithStubNode("v22.10.0")
			requireStubNodeVersion(path, "v22.10.0")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Status).To(Equal("needs_prerequisite"), "node_below_minimum must outrank the simultaneous policy_missing gap")
			Expect(gapCodes(doc)).To(Equal([]string{"policy_missing", "node_below_minimum"}), "both independently discoverable gaps must be reported, never hidden by precedence")
			Expect(nextActionKinds(doc)).To(ContainElements("author_policy", "install_node"))
		})
	})

	When("the repository cannot be read at the resolved revision (a corrupt/missing committed blob)", func() {
		It("exits 1 and never emits a readiness document, rather than reporting a confident but false verdict", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			corruptCommittedBlob(repo, "package.json")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(1), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "an operational failure must never emit a readiness JSON document")
			Expect(string(stderr)).NotTo(BeEmpty())
		})
	})

	When("the repository cannot resolve a committed subtree at the resolved revision (a corrupt/missing tree object)", func() {
		It("exits 1 and never emits a readiness document, rather than reporting the committed policy file as missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "config/project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			corruptCommittedTree(repo, "config")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "config/project.json", "--format", "json")
			Expect(exitCode).To(Equal(1), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "an operational failure must never emit a readiness JSON document")
			Expect(string(stderr)).NotTo(BeEmpty())
		})
	})

	When("HEAD has a committed project.json policy larger than the git-read size budget", func() {
		It("reports policy fail/policy_invalid and exits 0, treating the oversized (customer-fixable) file as a gap rather than an operational failure", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "project.json", strings.Repeat("a", (1<<20)+16))

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("fail"))
			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(doc.Status).To(Equal("needs_policy"))
		})
	})

	When("Node is on PATH but `node --version` hangs indefinitely", func() {
		It("still exits within a bounded wall clock, reporting node fail/node_below_minimum with a timed-out found_version rather than hanging forever", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithHangingNode()

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_below_minimum"))
			Expect(doc.Checks.Node.FoundVersion).To(Equal("timed out"))
			Expect(doc.Status).To(Equal("needs_prerequisite"))
		})
	})

	When("Node resolves and runs but prints output that is not a parsable version string", func() {
		It("reports node fail/node_below_minimum with the raw observed output, not node_missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("weird-build-2024")
			requireStubNodeVersion(path, "weird-build-2024")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_below_minimum"))
			Expect(doc.Checks.Node.FoundVersion).To(Equal("weird-build-2024"))
			Expect(doc.Status).To(Equal("needs_prerequisite"))
		})
	})

	DescribeTable("invalid --check-project argument combinations exit 2 for the specific reason validateCheckProjectFlags reports",
		func(args []string, wantMessage string) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, args...)
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring(wantMessage))
		},
		Entry("missing --baseline and --project-language",
			[]string{"--check-project"},
			"coach: --check-project requires --baseline"),
		Entry("missing --baseline",
			[]string{"--check-project", "--project-language", "typescript"},
			"coach: --check-project requires --baseline"),
		Entry("missing --project-language",
			[]string{"--baseline", "--check-project"},
			`coach: --check-project requires --project-language typescript (got "go")`),
		Entry("--project-language go is not typescript",
			[]string{"--baseline", "--check-project", "--project-language", "go"},
			`coach: --check-project requires --project-language typescript (got "go")`),
		Entry("duplicate --check-project",
			[]string{"--baseline", "--check-project", "--project-language", "typescript", "--check-project"},
			"coach: --check-project may only be provided once"),
		Entry("--check-project cannot be combined with --scope",
			[]string{"--baseline", "--check-project", "--project-language", "typescript", "--scope", "all"},
			"coach: --check-project cannot be combined with --scope"),
		Entry("--project-config is an absolute path",
			[]string{"--baseline", "--check-project", "--project-language", "typescript", "--project-config", "/etc/passwd"},
			`coach: --project-config "/etc/passwd" is invalid: path must be a non-empty repository-relative path`),
		Entry("--project-config escapes the repository via ..",
			[]string{"--baseline", "--check-project", "--project-language", "typescript", "--project-config", "../../etc/passwd"},
			`coach: --project-config "../../etc/passwd" is invalid: path must be normalized and remain inside the repository`),
	)
})
