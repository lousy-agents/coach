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

type readinessRootFindingDoc struct {
	Root    string `json:"root"`
	Version string `json:"version"`
}

type readinessCheckDoc struct {
	State             string                    `json:"state"`
	Code              string                    `json:"code"`
	Version           string                    `json:"version"`
	ExpectedVersion   string                    `json:"expected_version"`
	FoundVersion      string                    `json:"found_version"`
	SupportedVersions []string                  `json:"supported_versions"`
	RootFindings      []readinessRootFindingDoc `json:"root_findings"`
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
		ProjectShape   readinessCheckDoc `json:"project_shape"`
		Policy         readinessCheckDoc `json:"policy"`
		Node           readinessCheckDoc `json:"node"`
		Compiler       readinessCheckDoc `json:"compiler"`
		PackageManager readinessCheckDoc `json:"package_manager"`
	} `json:"checks"`
	Gaps []struct {
		Code string `json:"code"`
	} `json:"gaps"`
	Warnings []struct {
		Code              string `json:"code"`
		FoundMajor        int    `json:"found_major"`
		TestedMajor       int    `json:"tested_major"`
		FloorMajor        int    `json:"floor_major"`
		DeclaredVersion   string `json:"declared_version"`
		FoundVersion      string `json:"found_version"`
		DeclarationOrigin string `json:"declaration_origin"`
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
// reporting version, with every directory containing a real node/npm/mise
// executable removed so the stub is the only "node" the child process can
// resolve. mise is stripped too so resolveMiseGlobalCompiler deterministically
// finds no global-mise candidate, regardless of whether the host running the
// suite happens to have a real `mise` on PATH.
func pathWithStubNode(version string) string {
	return writeStubNodeScript(version) + string(os.PathListSeparator) + pathExcludingExecutables("node", "npm", "mise")
}

// pathWithoutNode returns a PATH with every node/npm/mise directory removed,
// so checkNodeReadiness deterministically reports node_missing regardless of
// the host's actual Node installation.
func pathWithoutNode() string {
	return pathExcludingExecutables("node", "npm", "mise")
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
// real node/npm/mise executable removed so the stub is the only "node" the
// child process can resolve.
func pathWithHangingNode() string {
	return writeHangingNodeScript() + string(os.PathListSeparator) + pathExcludingExecutables("node", "npm", "mise")
}

// stubMiseInvocationLog is the filename writeStubMiseScript's stub appends
// each invocation's argv to, one line per call.
const stubMiseInvocationLog = "mise-invocations.log"
const stubMiseCwdLog = "mise-probe-cwd.log"

// writeStubMiseScript writes an executable `mise` script into a fresh temp
// directory that always prints version regardless of its arguments,
// mirroring writeStubNodeScript, and separately records each invocation's
// argv into stubMiseInvocationLog in that same directory. resolveMiseGlobalCompiler
// shells out to whatever `mise` is first on the child process's PATH, so a
// spec that wants a specific, host-independent global-mise result must
// control PATH with a stub rather than depend on whether the host actually
// has mise installed; recording argv additionally lets a spec assert the
// frozen mise mechanic invoked only a read-only detection command, never a
// mutating one such as `mise install` or `mise use`.
func writeStubMiseScript(version string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubmise-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	installDir := filepath.Join(dir, "install")
	Expect(os.MkdirAll(filepath.Join(installDir, "node_modules", "typescript"), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(installDir, "node_modules", "typescript", "package.json"), []byte(fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", version)), 0o644)).To(Succeed())

	script := fmt.Sprintf("#!/bin/sh\necho \"$PWD\" >> %q\necho \"$@\" >> %q\nif [ \"$1\" = \"where\" ]; then echo %q; exit 0; fi\necho %s\n", filepath.Join(dir, stubMiseCwdLog), filepath.Join(dir, stubMiseInvocationLog), installDir, version)
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// readStubMiseInvocations reads the argv line(s) writeStubMiseScript's stub
// recorded for miseDir, one entry per invocation.
func readStubMiseInvocations(miseDir string) []string {
	data, err := os.ReadFile(filepath.Join(miseDir, stubMiseInvocationLog))
	Expect(err).NotTo(HaveOccurred(), "expected the stub mise at %s to have recorded at least one invocation", miseDir)
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func readStubMiseCwds(miseDir string) []string {
	data, err := os.ReadFile(filepath.Join(miseDir, stubMiseCwdLog))
	Expect(err).NotTo(HaveOccurred(), "expected the stub mise at %s to have recorded probe working directories", miseDir)
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// pathWithStubNodeAndMise returns a PATH whose first two entries are a stub
// `node` reporting nodeVersion and a stub `mise` reporting miseVersion
// (regardless of its arguments), with every directory containing a real
// node/npm/mise executable removed, plus the stub mise's own directory so a
// spec can inspect its recorded invocations via readStubMiseInvocations.
func pathWithStubNodeAndMise(nodeVersion, miseVersion string) (path, miseDir string) {
	miseDir = writeStubMiseScript(miseVersion)
	path = writeStubNodeScript(nodeVersion) + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingExecutables("node", "npm", "mise")
	return path, miseDir
}

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript", func() {
	When("HEAD has a TypeScript-shaped project (package.json) but no project.json policy, and Node is a supported major", func() {
		It("exits 0, reports status needs_policy, policy fail/policy_missing, compiler pass, and an author_policy next action (JSON)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(doc.Checks.PackageManager.State).To(Equal("not_checked"))
			Expect(doc.Checks.PackageManager.Code).To(BeEmpty())
			Expect(gapCodes(doc)).NotTo(ContainElement("package_manager_ambiguous"))
			Expect(gapCodes(doc)).NotTo(ContainElement("package_manager_config_unverifiable"))
			Expect(doc.Checks.Node.State).To(Equal("pass"), "the stubbed, supported Node major must report pass deterministically")
			Expect(gapCodes(doc)).To(ContainElement("policy_missing"))
			Expect(nextActionKinds(doc)).To(ContainElement("author_policy"))
		})

		It("renders the same status, gaps, and next actions in the default text format", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("status: needs_policy"))
			Expect(text).To(ContainSubstring("policy: fail (policy_missing)"))
			Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
			Expect(text).To(ContainSubstring("package_manager: not_checked"))
			Expect(text).To(ContainSubstring("author_policy"))
		})
	})

	When("HEAD has a committed, valid project.json policy plus tsconfig.json and package.json, and Node is a supported major", func() {
		It("reports the policy check as pass and, with every other check clean, an overall status of ready", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("pass"))
			Expect(doc.Checks.Policy.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(doc.Checks.PackageManager.State).To(Equal("not_checked"))
			Expect(doc.Checks.PackageManager.Code).To(BeEmpty())
			Expect(gapCodes(doc)).To(BeEmpty())
			Expect(doc.Status).To(Equal("ready"))
		})

		It("reports the same policy pass when --project-config is omitted, reading the default project.json at HEAD", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			explicitStdout, explicitStderr, explicitExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(explicitExit).To(Equal(0), "stderr: %s", explicitStderr)
			omittedStdout, omittedStderr, omittedExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(omittedExit).To(Equal(0), "stderr: %s", omittedStderr)

			var explicitDoc, omittedDoc readinessResultDoc
			Expect(json.Unmarshal(explicitStdout, &explicitDoc)).To(Succeed(), "stdout: %s", explicitStdout)
			Expect(json.Unmarshal(omittedStdout, &omittedDoc)).To(Succeed(), "stdout: %s", omittedStdout)
			Expect(omittedDoc).To(Equal(explicitDoc))
			Expect(omittedDoc.Checks.Policy.State).To(Equal("pass"))
			Expect(omittedDoc.Status).To(Equal("ready"))
		})
	})

	When("coach is invoked from a committed subdirectory of the repository rather than the repository root", func() {
		It("resolves package.json and the --project-config policy against the repository root, reporting the same ready verdict as from the root", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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

	When("node_modules is gitignored and present only as untracked worktree files", func() {
		It("does not list those paths as relevant dirty worktree changes", func() {
			repo := newTempGitRepo()
			commitFile(repo, ".gitignore", "node_modules\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			Expect(os.MkdirAll(filepath.Join(repo, "node_modules", "pkg"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "node_modules", "pkg", "index.js"), []byte("module.exports = {}\n"), 0o644)).To(Succeed())

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Status).To(Equal("ready"))
			Expect(doc.DirtyWorktree.RelevantChanges).To(BeFalse())
			Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(HavePrefix("node_modules")))
		})
	})

	When("HEAD has a committed directory named package.json and no package.json blob", func() {
		It("reports project_shape fail/unsupported_repository_shape rather than treating the tree as a project manifest", func() {
			repo := newTempGitRepo()
			Expect(os.Mkdir(filepath.Join(repo, "package.json"), 0o755)).To(Succeed())
			commitFile(repo, "package.json/inner.txt", "not a manifest\n")

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.ProjectShape.State).To(Equal("fail"))
			Expect(doc.Checks.ProjectShape.Code).To(Equal("unsupported_repository_shape"))
			Expect(doc.Status).To(Equal("outside_support"))
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
			By("relying on a locatable project mise.toml compiler so the compiler check passes cleanly, isolating this assertion to the roots budget")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, _ := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)
			Expect(text).To(ContainSubstring("status: ready_with_limits"))
			Expect(text).To(ContainSubstring("node: pass (node_untested) version=v26.0.0"))
			Expect(text).To(ContainSubstring("node_untested (found_major=26 tested_major=24 floor_major=24)"))
		})
	})

	When("Node resolves to exactly this build's tested major", func() {
		It("reports the node check as pass with no code, warning, or gap", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
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
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

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

// writeWorktreeFile writes name with contents directly into the worktree at
// repo without committing or `git add`ing it. The compiler check reads
// package.json/mise.toml/node_modules as host-readiness state of the
// worktree, never the Git snapshot, so these fixtures deliberately stay
// uncommitted -- proving the resolver reads the worktree directly rather
// than depending on anything reaching HEAD.
func writeWorktreeFile(repo, name, contents string) {
	full := filepath.Join(repo, name)
	Expect(os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
	Expect(os.WriteFile(full, []byte(contents), 0o644)).To(Succeed())
}

func writeInstalledTypescript(repo, version string) {
	writeInstalledTypescriptUnder(repo, ".", version)
}

func writeInstalledTypescriptUnder(repo, relDir, version string) {
	if _, err := os.Stat(filepath.Join(repo, ".gitignore")); err != nil {
		commitFile(repo, ".gitignore", "node_modules\n")
	}
	manifest := "node_modules/typescript/package.json"
	if relDir != "." && relDir != "" {
		manifest = relDir + "/" + manifest
	}
	writeWorktreeFile(repo, manifest, fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", version))
}

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: compiler resolution", func() {
	When("the committed package.json declares a unique exact typescript version", func() {
		It("reports the compiler check as pass with the resolved version, from the project manifest origin", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
		})
	})

	When("package.json declares no typescript version, but the worktree's project mise.toml pins a unique exact npm:typescript version", func() {
		It("reports the compiler check as pass with that version, from the project mise origin", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, _ := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
		})
	})

	When("neither package.json nor a project mise.toml name a typescript version, but the host's global mise configuration pins a unique exact npm:typescript version", func() {
		It("reports the compiler check as pass with that version, from the global mise origin, using only a read-only mise invocation", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")

			path, miseDir := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(readStubMiseInvocations(miseDir)).To(Equal([]string{
				"config get tools.npm:typescript -g",
				"where npm:typescript@7.0.2",
			}), "the frozen global-mise mechanic must invoke only read-only detection and location commands, never mise install/use or any other mutating subcommand")
		})
	})

	When("package.json declares a unique exact typescript version that is not installed, even though project mise pins a different exact version", func() {
		It("reports fail/typescript_compiler_missing rather than passing the undeployed pin or silently selecting the mise pin", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"9.9.9\"\n")

			path, _ := pathWithStubNodeAndMise("v24.9.9", "9.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(doc.Checks.Compiler.Version).NotTo(Equal("7.0.2"), "a declared-but-not-installed pin is not a selected compiler")
			Expect(doc.Checks.Compiler.Version).NotTo(Equal("9.9.9"), "a lower-precedence origin must not be selected after a higher origin produced a candidate")
			Expect(gapCodes(doc)).To(ContainElement("typescript_compiler_missing"))
		})
	})

	When("package.json declares a unique exact typescript version and the worktree's project mise.toml also pins a different exact version", func() {
		It("resolves from the project manifest origin, pinning that project outranks project mise rather than merely being the only candidate present", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"9.9.9\"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"), "the project manifest origin must outrank a conflicting project mise.toml candidate")
		})
	})

	When("the worktree's project mise.toml pins a unique exact version and the host's global mise configuration pins a different exact version", func() {
		It("resolves from the project mise origin, pinning that project mise outranks global mise rather than merely being the only candidate present", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, _ := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"), "the project mise origin must outrank a conflicting global mise candidate")
		})
	})

	When("package.json's declared exact typescript version disagrees with the version actually installed under node_modules/typescript", func() {
		It("reports the compiler check as fail/typescript_version_mismatch with both versions recorded", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			writeWorktreeFile(repo, "node_modules/typescript/package.json", `{"name":"typescript","version":"5.4.0"}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_mismatch"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(Equal("7.0.2"))
			Expect(doc.Checks.Compiler.FoundVersion).To(Equal("5.4.0"))
			Expect(doc.Checks.Compiler.SupportedVersions).To(Equal([]string{"7.0.2"}))
			Expect(gapCodes(doc)).To(ContainElement("typescript_version_mismatch"))
			Expect(nextActionKinds(doc)).To(ContainElement("prepare_compiler"))
		})
	})

	When("package.json declares two different exact typescript versions across dependencies and devDependencies", func() {
		It("reports the compiler check as fail/typescript_version_conflict rather than silently picking one", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","dependencies":{"typescript":"7.0.2"},"devDependencies":{"typescript":"5.4.0"}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_conflict"))
			Expect(gapCodes(doc)).To(ContainElement("typescript_version_conflict"))
		})
	})

	When("a js/semantics-shaped repository has no top-level package.json and the policy names the nested project root that pins an exact installed compiler", func() {
		It("reports checks.compiler.state=pass from that nested manifest rather than typescript_compiler_missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["js/semantics"]}`+"\n")
			commitFile(repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "js/semantics/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			writeInstalledTypescriptUnder(repo, "js/semantics", "7.0.2")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "nested-only fixture must not have a top-level package.json; that would exercise the already-green worktree-top origin")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("pass"), "the nested policy root must itself be valid so a compiler miss cannot be blamed on policy")
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "the nested js/semantics package.json pin must certify the compiler, got state=%s code=%s version=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, doc.Checks.Compiler.Version, stdout)
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
		})
	})

	When("the policy selects roots [\".\"] and an exact installed compiler exists only under a nested directory", func() {
		It("reports fail/typescript_compiler_missing rather than walking down to certify the nested pin", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescriptUnder(repo, "js/semantics", "7.0.2")

			_, statErr := os.Stat(filepath.Join(repo, "js/semantics/package.json"))
			Expect(statErr).NotTo(HaveOccurred(), "walk-down fixture must keep a nested pin that a walk-down resolver would find")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.ProjectShape.State).To(Equal("pass"), "a top-level package.json must keep this on the compiler check, not unsupported_repository_shape")
			Expect(doc.Checks.Compiler.State).To(Equal("fail"), "roots [\".\"] must not walk down to js/semantics, got state=%s code=%s version=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, doc.Checks.Compiler.Version, stdout)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(doc.Checks.Compiler.Version).NotTo(Equal("7.0.2"), "the nested pin must not be selected as a passing compiler")
			Expect(gapCodes(doc)).To(ContainElement("typescript_compiler_missing"))
		})
	})

	When("a js/semantics-shaped repository has no top-level package.json and the policy names a subdirectory of the nested project whose package.json lives only in the parent", func() {
		It("reports checks.compiler.state=pass from the parent manifest rather than typescript_compiler_missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["js/semantics/src"]}`+"\n")
			commitFile(repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "js/semantics/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "js/semantics/src/index.ts", "export const x = 1;\n")
			writeInstalledTypescriptUnder(repo, "js/semantics", "7.0.2")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "walk-up fixture must not have a top-level package.json; that would exercise the already-green worktree-top origin")
			_, statErr = os.Stat(filepath.Join(repo, "js/semantics/src/package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "walk-up fixture must not place package.json at the selected root; that would exercise the already-green at-root origin")
			_, statErr = os.Stat(filepath.Join(repo, "js/semantics/package.json"))
			Expect(statErr).NotTo(HaveOccurred(), "walk-up fixture must keep the pin strictly above the selected root")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("pass"), "the nested policy root must itself be valid so a compiler miss cannot be blamed on policy")
			Expect(doc.Checks.ProjectShape.State).To(Equal("pass"), "walk-up to the parent package.json must certify project_shape the same way it certifies the compiler, got state=%s code=%s stdout=%s", doc.Checks.ProjectShape.State, doc.Checks.ProjectShape.Code, stdout)
			Expect(doc.Status).NotTo(Equal("outside_support"), "a nested project whose package.json lives only above the selected root must not be outside_support, got status=%s stdout=%s", doc.Status, stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "the parent js/semantics package.json pin must certify the compiler via walk-up, got state=%s code=%s version=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, doc.Checks.Compiler.Version, stdout)
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
		})
	})

	When("two policy roots include one nested exact installed compiler and one nested root with no package.json, and there is no top-level package.json", func() {
		It("reports fail/typescript_version_conflict rather than silently certifying the pinned root", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["apps/web","apps/api"]}`+"\n")
			commitFile(repo, "apps/web/package.json", `{"name":"web","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "apps/api/index.ts", "export const api = 1;\n")
			writeInstalledTypescriptUnder(repo, "apps/web", "7.0.2")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "mixed pass+empty must not share a top-level package.json")
			_, statErr = os.Stat(filepath.Join(repo, "apps/api/package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "the empty root must have no package.json; a second pin would exercise the already-green two-pin conflict")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"), "a selected root with no manifest must not be skipped, got state=%s code=%s version=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, doc.Checks.Compiler.Version, stdout)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_conflict"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(BeEmpty(), "conflict omits expected_version; root_findings is the sole machine-readable surface")
			Expect(doc.Checks.Compiler.FoundVersion).To(BeEmpty(), "conflict omits found_version; root_findings is the sole machine-readable surface")
			Expect(doc.Checks.Compiler.Version).NotTo(Equal("7.0.2"), "the pinned root must not be selected as a passing compiler")
			Expect(doc.Checks.Compiler.RootFindings).To(Equal([]readinessRootFindingDoc{
				{Root: "apps/web", Version: "7.0.2"},
				{Root: "apps/api"},
			}), "each selected root's finding must be named, got %+v stdout=%s", doc.Checks.Compiler.RootFindings, stdout)
			Expect(gapCodes(doc)).To(ContainElement("typescript_version_conflict"))
		})
	})

	When("two policy roots pin disagreeing exact typescript versions in distinct nested manifests with no top-level package.json", func() {
		It("reports fail/typescript_version_conflict naming each root's version in root_findings", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["apps/web","apps/api"]}`+"\n")
			commitFile(repo, "apps/web/package.json", `{"name":"web","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "apps/api/package.json", `{"name":"api","version":"1.0.0","devDependencies":{"typescript":"5.4.0"}}`+"\n")
			writeInstalledTypescriptUnder(repo, "apps/web", "7.0.2")
			writeInstalledTypescriptUnder(repo, "apps/api", "5.4.0")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "two-root disagreement must not share a top-level package.json")
			webManifest, err := os.ReadFile(filepath.Join(repo, "apps/web/package.json"))
			Expect(err).NotTo(HaveOccurred())
			apiManifest, err := os.ReadFile(filepath.Join(repo, "apps/api/package.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(webManifest).NotTo(Equal(apiManifest), "two-root disagreement must be two distinct manifests, not one package.json reached twice")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"), "disagreeing per-root pins must fail closed, got state=%s code=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, stdout)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_conflict"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(BeEmpty(), "conflict omits expected_version; root_findings is the sole machine-readable surface, got expected=%q found=%q", doc.Checks.Compiler.ExpectedVersion, doc.Checks.Compiler.FoundVersion)
			Expect(doc.Checks.Compiler.FoundVersion).To(BeEmpty(), "conflict omits found_version")
			Expect(doc.Checks.Compiler.RootFindings).To(Equal([]readinessRootFindingDoc{
				{Root: "apps/web", Version: "7.0.2"},
				{Root: "apps/api", Version: "5.4.0"},
			}), "each selected root's finding must be named, got %+v stdout=%s", doc.Checks.Compiler.RootFindings, stdout)
			Expect(gapCodes(doc)).To(ContainElement("typescript_version_conflict"))

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(textStdout)).To(ContainSubstring("apps/web"), "text remediation must name the web root, got %s", textStdout)
			Expect(string(textStdout)).To(ContainSubstring("apps/api"), "text remediation must name the api root, got %s", textStdout)
		})
	})

	When("two policy roots pin the same exact installed typescript version in distinct nested manifests with no top-level package.json", func() {
		It("reports checks.compiler.state=pass rather than typescript_version_conflict", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["apps/web","apps/api"]}`+"\n")
			commitFile(repo, "apps/web/package.json", `{"name":"web","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "apps/api/package.json", `{"name":"api","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescriptUnder(repo, "apps/web", "7.0.2")
			writeInstalledTypescriptUnder(repo, "apps/api", "7.0.2")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "two-root agreement must not share a top-level package.json")
			webManifest, err := os.ReadFile(filepath.Join(repo, "apps/web/package.json"))
			Expect(err).NotTo(HaveOccurred())
			apiManifest, err := os.ReadFile(filepath.Join(repo, "apps/api/package.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(webManifest).NotTo(Equal(apiManifest), "two-root agreement must be two distinct manifests that happen to share a pin")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "agreeing per-root pins must pass, got state=%s code=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, stdout)
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
		})
	})

	When("no exact compiler candidate exists in the project manifest, project mise config, or global mise config", func() {
		It("reports the compiler check as fail/typescript_compiler_missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(Equal("7.0.2"))
			Expect(gapCodes(doc)).To(ContainElement("typescript_compiler_missing"))
		})
	})

	When("package.json is not valid JSON, even though the worktree's project mise.toml would otherwise resolve a compiler cleanly", func() {
		It("rejects the unreadable manifest as fail/typescript_compiler_missing rather than silently falling through to mise or crashing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name": "example", "devDependencies": {`+"\n")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"9.9.9\"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(doc.Checks.Compiler.Version).NotTo(Equal("9.9.9"), "an unreadable project manifest must never silently fall through to a lower-precedence origin")
		})
	})

	When("package.json exists but is unreadable (permission-denied), even though the worktree's project mise.toml would otherwise resolve a compiler cleanly", func() {
		It("rejects the unreadable manifest as fail/typescript_compiler_missing rather than silently falling through to mise", func() {
			if os.Geteuid() == 0 {
				Skip("cannot exercise a permission-denied read while running as root")
			}
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"9.9.9\"\n")

			packageJSONPath := filepath.Join(repo, "package.json")
			Expect(os.Chmod(packageJSONPath, 0o000)).To(Succeed())
			DeferCleanup(func() { os.Chmod(packageJSONPath, 0o644) })

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(doc.Checks.Compiler.Version).NotTo(Equal("9.9.9"), "an unreadable project manifest must never silently fall through to a lower-precedence origin")
		})
	})

	When("the installed compiler is an exact 5.x version from its own package.json", func() {
		It("reports fail/typescript_version_mismatch with supported_versions [7.0.2] rather than pass", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"5.4.0"}}`+"\n")
			writeInstalledTypescript(repo, "5.4.0")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_mismatch"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(Equal("7.0.2"))
			Expect(doc.Checks.Compiler.FoundVersion).To(Equal("5.4.0"))
			Expect(doc.Checks.Compiler.SupportedVersions).To(Equal([]string{"7.0.2"}))
			Expect(gapCodes(doc)).To(ContainElement("typescript_version_mismatch"))

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(textStdout)).To(ContainSubstring("7.0.2"), "gap text must name the supported set, got %s", textStdout)
		})
	})

	When("package.json declares a ^7 range, 7.0.2 is installed at the project origin, and project mise supplies 7.0.2", func() {
		It("disqualifies the project origin and reports pass from mise with a compiler_declaration_mismatch warning", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"^7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, _ := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "mise origin must supply the supported compiler after the range declaration is disqualified, got state=%s code=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, stdout)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(doc.Status).To(Equal("ready_with_limits"))
			Expect(gapCodes(doc)).NotTo(ContainElement("compiler_declaration_mismatch"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("prepare_compiler"))
			Expect(doc.Warnings).To(ContainElement(HaveField("Code", "compiler_declaration_mismatch")))
			var warning struct {
				Code              string
				DeclaredVersion   string
				FoundVersion      string
				DeclarationOrigin string
			}
			for _, w := range doc.Warnings {
				if w.Code == "compiler_declaration_mismatch" {
					warning.Code = w.Code
					warning.DeclaredVersion = w.DeclaredVersion
					warning.FoundVersion = w.FoundVersion
					warning.DeclarationOrigin = w.DeclarationOrigin
				}
			}
			Expect(warning.DeclaredVersion).To(Equal("^7.0.2"))
			Expect(warning.FoundVersion).To(Equal("7.0.2"))
			Expect(warning.DeclarationOrigin).To(Equal("manifest"))
		})
	})

	When("the worktree mise.toml carries an env exec template that would write a sentinel", func() {
		It("produces no side effect during --check-project because mise probes use a neutral working directory", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			sentinel := filepath.Join(repo, "mise-exec-side-effect")
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[env]\nSIDE_EFFECT = \"{{ exec(command='touch %s') }}\"\n", sentinel))

			path, miseDir := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty())

			_, statErr := os.Stat(sentinel)
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "mise exec template must not run during the fit check")
			for _, cwd := range readStubMiseCwds(miseDir) {
				Expect(cwd).NotTo(Equal(repo), "mise probes must not run with the analyzed repository as cwd, got %q", cwd)
			}
		})
	})
})
