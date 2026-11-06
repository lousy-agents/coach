package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

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

var _ = Describe("coach codesignal: the restore command printed for a missing committed policy is safe to paste, whatever the name", func() {
	pasteable := []string{`q"uote.json`, "`touch PWN`.json", "a;touch PWN;b.json", "-x.json", "*.json", ":(top)x.json", "!bang#hash.json", "HEAD~1.json", "it's.json", "my policy.json", "$(touch PWN).json"}

	DescribeTable("a Go scan, run through sh",
		func(name string) {
			repo := commitScannableGoRepo()
			commitThenDelete(repo, name)

			command := restoreCommandFrom([]byte(refusedScan(repo, name)))
			Expect(command).NotTo(BeEmpty(), "the error offers no command")
			output, exitCode := runInShell(repo, command)

			Expect(exitCode).To(Equal(0), "command=%s output=%s", command, output)
			Expect(filepath.Join(repo, "PWN")).NotTo(BeAnExistingFile(), "the shell ran part of the file name")
			Expect(filepath.Join(repo, name)).To(BeARegularFile())
			Expect(gitIn(repo, "status", "--porcelain")).To(BeEmpty(), "the restore changed more than the policy")
		},
		func(name string) string { return "the name " + name },
		Entry(nil, pasteable[0]), Entry(nil, pasteable[1]), Entry(nil, pasteable[2]), Entry(nil, pasteable[3]), Entry(nil, pasteable[4]),
		Entry(nil, pasteable[5]), Entry(nil, pasteable[6]), Entry(nil, pasteable[7]), Entry(nil, pasteable[8]), Entry(nil, pasteable[9]), Entry(nil, pasteable[10]),
	)

	DescribeTable("a TypeScript scan, run through sh",
		func(name string) {
			repo := commitScannableGoRepo()
			commitThenDelete(repo, name)

			command := restoreCommandFrom([]byte(refusedScan(repo, name, "--project-language", "typescript")))
			Expect(command).NotTo(BeEmpty(), "the error offers no command")
			output, exitCode := runInShell(repo, command)

			Expect(exitCode).To(Equal(0), "command=%s output=%s", command, output)
			Expect(filepath.Join(repo, "PWN")).NotTo(BeAnExistingFile(), "the shell ran part of the file name")
			Expect(filepath.Join(repo, name)).To(BeARegularFile())
		},
		func(name string) string { return "the name " + name },
		Entry(nil, pasteable[1]), Entry(nil, pasteable[2]), Entry(nil, pasteable[3]), Entry(nil, pasteable[5]), Entry(nil, pasteable[8]), Entry(nil, pasteable[9]), Entry(nil, pasteable[10]),
	)

	DescribeTable("a name with a control, format or separator character gets no command and none of it is printed raw",
		func(name string) {
			repo := commitScannableGoRepo()
			commitThenDelete(repo, name)

			for _, language := range []string{"go", "typescript"} {
				stderr := refusedScan(repo, name, "--project-language", language)

				Expect(stderr).NotTo(ContainSubstring("git restore"), language)
				Expect(stderr).To(ContainSubstring("not printed here"), language)
				for _, r := range stderr {
					Expect(r == '\n' || r == '\t' || (r >= 0x20 && r != 0x7f && r != 0x202e && r != 0x2028 && r != 0x200b && utf8.ValidRune(r))).To(BeTrue(), "raw %U reached the terminal (%s)", r, language)
				}
			}
		},
		func(name string) string {
			return "the name " + strings.NewReplacer("\x07", `\a`, "\n", `\n`, "\x7f", `\x7f`).Replace(name)
		},
		Entry(nil, "bell\x07.json"), Entry(nil, "nl\nname.json"), Entry(nil, "x\u202eY.json"), Entry(nil, "x\u2028y.json"), Entry(nil, "x\u200by.json"), Entry(nil, "del\x7f.json"),
	)

	It("--check-project prints no command for any of these names, only a restore_policy next action that is not executable", func() {
		for _, name := range []string{"$(touch PWN).json", "nl\nname.json", "a;touch PWN;b.json"} {
			repo := commitScannableGoRepo()
			commitThenDelete(repo, name)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--format", "json", "--project-config", name)

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stdout)).To(ContainSubstring(`"kind":"restore_policy","executable":false`))
			Expect(string(stdout) + string(stderr)).NotTo(ContainSubstring("git restore"))
			Expect(filepath.Join(repo, "PWN")).NotTo(BeAnExistingFile())
		}
	})
})

var _ = Describe("coach codesignal: a committed policy that is evil is refused with a reason, and promptly", func() {
	DescribeTable("the committed bytes are not a usable policy",
		func(content, reason string) {
			repo := commitScannableGoRepo()
			commitFile(repo, explicitPolicyPath, content)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
			Expect(string(stderr)).To(ContainSubstring(reason))
		},
		Entry("random binary", "\x00\xff\xfe\x80binary\x00\x01", "document is not valid JSON"),
		Entry("zero bytes", "", "document is not valid JSON"),
		Entry("a byte-order mark before the JSON", "\xef\xbb\xbf"+singleRootPolicyJSON, "document is not valid JSON"),
		Entry("200000 levels of nesting", strings.Repeat("[", 200000)+strings.Repeat("]", 200000), "document is not valid JSON"),
		Entry("a duplicated object key", `{"schema_version":"1","roots":["."],"roots":["../../etc"]}`, `duplicate object key "roots"`),
		Entry("a root that leaves the repository", `{"schema_version":"1","roots":["../../etc"]}`, `root "../../etc": must be a normalized repository-relative path`),
	)

	It("refuses a committed policy over the output budget within seconds, not at the git time bound", func() {
		// well past the budget: git can only be left blocked writing when more is unread than a pipe holds
		repo := commitScannableGoRepo()
		commitFile(repo, explicitPolicyPath, strings.Repeat(" ", 2<<20)+singleRootPolicyJSON)

		start := time.Now()
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("git stdout exceeded 1048576-byte budget"))
		Expect(time.Since(start)).To(BeNumerically("<", 10*time.Second), "the refusal waited for git's 30-second bound")
	})
})

var _ = Describe("coach codesignal: git states around an explicit --project-config", func() {
	It("still scans when a stale .git/index.lock is present and the policy is intact", func() {
		repo := commitExplicitPolicyRepo()
		Expect(os.WriteFile(filepath.Join(repo, ".git", "index.lock"), nil, 0o644)).To(Succeed())

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(stdout).NotTo(BeEmpty())
	})

	It("counts an intent-to-add entry for a deleted policy as no entry: no restore command, authoring offered", func() {
		repo := commitScannableGoRepo()
		Expect(os.MkdirAll(filepath.Join(repo, "policies"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, explicitPolicyPath), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
		gitIn(repo, "add", "-N", "--", explicitPolicyPath)
		Expect(os.Remove(filepath.Join(repo, explicitPolicyPath))).To(Succeed())

		stderr := expectRefusedWith(repo, explicitPolicyPath, "project_config_missing", "was not found on disk")

		Expect(stderr).NotTo(ContainSubstring("git restore"), "an entry with no content has nothing to restore")
		Expect(stderr).To(ContainSubstring("--suggest-project-config"))
	})

	It("scans from a detached HEAD", func() {
		repo := commitExplicitPolicyRepo()
		gitIn(repo, "checkout", "-q", "--detach", "HEAD")

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(stdout).NotTo(BeEmpty())
	})

	It("scans a linked worktree, and a policy deleted there is restored there and nowhere else", func() {
		repo := commitExplicitPolicyRepo()
		linked := repo + "-linked"
		gitIn(repo, "worktree", "add", "-q", linked)
		DeferCleanup(func() { _ = os.RemoveAll(linked) })

		stdout, stderr, exitCode := runCoachBinary(commandPath, linked, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(stdout).NotTo(BeEmpty())

		Expect(os.Remove(filepath.Join(linked, explicitPolicyPath))).To(Succeed())
		command := restoreCommandFrom([]byte(refusedScan(linked, explicitPolicyPath)))
		Expect(command).NotTo(BeEmpty())
		output, restoreExit := runInShell(linked, command)

		Expect(restoreExit).To(Equal(0), "command=%s output=%s", command, output)
		Expect(filepath.Join(linked, explicitPolicyPath)).To(BeARegularFile())
		Expect(gitIn(repo, "status", "--porcelain")).To(BeEmpty(), "the main worktree was touched")
	})

	It("reports an operational failure, not a policy error, with no commits, outside a repository, in a bare repository and for an unknown --base", func() {
		unborn := GinkgoT().TempDir()
		gitIn(unborn, "init", "-q")
		bareSource := commitExplicitPolicyRepo()
		bare := bareSource + ".bare"
		Expect(exec.Command("git", "clone", "-q", "--bare", bareSource, bare).Run()).To(Succeed())
		DeferCleanup(func() { _ = os.RemoveAll(bare) })

		for _, scene := range []struct{ name, dir, reason string }{
			{"no commits", unborn, "HEAD is not readable"},
			{"outside a repository", GinkgoT().TempDir(), "is not inside a Git worktree"},
			{"a bare repository", bare, "is not inside a Git worktree"},
		} {
			stdout, stderr, exitCode := runCoachBinary(commandPath, scene.dir, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath)
			Expect(exitCode).To(Equal(1), scene.name+": "+string(stderr))
			Expect(stdout).To(BeEmpty(), scene.name)
			Expect(string(stderr)).To(ContainSubstring(scene.reason), scene.name)
		}

		repo := commitExplicitPolicyRepo()
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--base", "no-such-ref", "--project-config", explicitPolicyPath)
		Expect(exitCode).To(Equal(1), string(stderr))
		Expect(stdout).To(BeEmpty())
		Expect(string(stderr)).To(ContainSubstring(`--base "no-such-ref" cannot be resolved to a commit`))

		stdout, stderr, exitCode = runCoachBinary(commandPath, repo, nil, "codesignal", "--base", "--output=x", "--project-config", explicitPolicyPath)
		Expect(exitCode).To(Equal(1), string(stderr))
		Expect(stdout).To(BeEmpty())
		Expect(string(stderr)).To(ContainSubstring("cannot be resolved to a commit"))
	})

	It("refuses a policy path that is a submodule, and one that runs into a submodule's checkout", func() {
		repo := commitScannableGoRepo()
		inner := GinkgoT().TempDir()
		gitIn(inner, "init", "-q")
		Expect(os.WriteFile(filepath.Join(inner, "coach.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
		gitIn(inner, "add", "-A")
		innerCommit := exec.Command("git", "commit", "-q", "-m", "inner")
		innerCommit.Dir, innerCommit.Env = inner, commitEnv
		Expect(innerCommit.Run()).To(Succeed())
		gitIn(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", inner, "vendor/sub")
		superCommit := exec.Command("git", "commit", "-q", "-m", "sub")
		superCommit.Dir, superCommit.Env = repo, commitEnv
		Expect(superCommit.Run()).To(Succeed())

		expectRefusedWith(repo, "vendor/sub", "project_config_invalid", "is not a regular file")
		expectRefusedWith(repo, "vendor/sub/coach.json", "project_config_invalid", "is not committed at revision")
		expectRefusedWith(repo, "vendor/sub/absent.json", "project_config_missing", "was not found on disk")
	})

	It("scans and restores in a SHA-256 repository", func() {
		repo := GinkgoT().TempDir()
		if out, err := exec.Command("git", "-C", repo, "init", "-q", "--object-format=sha256").CombinedOutput(); err != nil {
			Skip("this git cannot create a SHA-256 repository: " + string(out))
		}
		for name, content := range map[string]string{"a.go": minimalGoSource, explicitPolicyPath: singleRootPolicyJSON} {
			Expect(os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644)).To(Succeed())
		}
		gitIn(repo, "add", "-A")
		commit := exec.Command("git", "commit", "-q", "-m", "init")
		commit.Dir, commit.Env = repo, commitEnv
		Expect(commit.Run()).To(Succeed())

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(stdout).NotTo(BeEmpty())

		Expect(os.Remove(filepath.Join(repo, explicitPolicyPath))).To(Succeed())
		command := restoreCommandFrom([]byte(refusedScan(repo, explicitPolicyPath)))
		Expect(command).To(MatchRegexp(`--source=[0-9a-f]{64} `), "the revision is the full 64-hex object name")
		output, restoreExit := runInShell(repo, command)
		Expect(restoreExit).To(Equal(0), "command=%s output=%s", command, output)
		Expect(filepath.Join(repo, explicitPolicyPath)).To(BeARegularFile())
	})

	It("resolves either Unicode spelling of a committed name on a filesystem that ignores normalization", func() {
		composed := "caf\u00e9.json"
		decomposed := "cafe\u0301.json"
		probe := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(probe, composed), nil, 0o644)).To(Succeed())
		if _, err := os.Stat(filepath.Join(probe, decomposed)); err != nil {
			Skip("this filesystem keeps composed and decomposed spellings apart")
		}
		for _, spelling := range []struct{ committed, passed string }{{composed, decomposed}, {decomposed, composed}} {
			repo := commitScannableGoRepo()
			commitFile(repo, spelling.committed, singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", spelling.passed, "--format=json")

			Expect(exitCode).To(Equal(0), "committed %q, passed %q: %s", spelling.committed, spelling.passed, stderr)
			Expect(stdout).NotTo(BeEmpty())
		}
	})
})

var _ = Describe("coach codesignal: a hostile git configuration or environment executes nothing", func() {
	script := func(marker string) string {
		path := filepath.Join(GinkgoT().TempDir(), "run.sh")
		Expect(os.WriteFile(path, []byte("#!/bin/sh\ntouch '"+marker+"'\nprintf '\\0'\n"), 0o755)).To(Succeed())
		return path
	}

	It("runs no core.fsmonitor program, from the repository config or from GIT_CONFIG_COUNT", func() {
		marker := filepath.Join(GinkgoT().TempDir(), "ran")
		program := script(marker)
		repo := commitExplicitPolicyRepo()
		gitIn(repo, "config", "core.fsmonitor", program)
		env := append(os.Environ(), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.fsmonitor", "GIT_CONFIG_VALUE_0="+program)

		_, stderr, exitCode := runCoachBinary(commandPath, repo, env, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(os.Remove(filepath.Join(repo, explicitPolicyPath))).To(Succeed())
		refusedViaEnv := func() {
			_, _, code := runCoachBinary(commandPath, repo, env, "codesignal", "--baseline", "--project-config", explicitPolicyPath)
			Expect(code).To(Equal(2))
		}
		refusedViaEnv()

		Expect(marker).NotTo(BeAnExistingFile(), "git ran the core.fsmonitor program the repository's configuration names")
	})

	It("runs no hook, external diff, textconv driver, pager, askpass or ssh program", func() {
		marker := filepath.Join(GinkgoT().TempDir(), "ran")
		program := script(marker)
		repo := commitExplicitPolicyRepo()
		Expect(os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("*.go diff=evil\n"), 0o644)).To(Succeed())
		gitIn(repo, "add", ".gitattributes")
		attrs := exec.Command("git", "commit", "-q", "-m", "attrs")
		attrs.Dir, attrs.Env = repo, commitEnv
		Expect(attrs.Run()).To(Succeed())
		// installed after the setup commit, which would otherwise run them itself
		hooks := filepath.Join(repo, ".git", "hooks")
		for _, hook := range []string{"post-checkout", "reference-transaction", "post-index-change", "post-merge"} {
			Expect(os.WriteFile(filepath.Join(hooks, hook), []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o755)).To(Succeed())
		}
		gitIn(repo, "config", "diff.evil.textconv", "touch '"+marker+"'; cat")
		gitIn(repo, "config", "diff.external", program)
		Expect(os.WriteFile(filepath.Join(repo, "a.go"), []byte(minimalGoSource+"// changed\n"), 0o644)).To(Succeed())
		env := append(os.Environ(), "GIT_EXTERNAL_DIFF="+program, "GIT_PAGER="+program, "PAGER="+program, "GIT_ASKPASS="+program, "GIT_SSH_COMMAND="+program, "GIT_EDITOR="+program)

		for _, args := range [][]string{
			{"codesignal", "--base", "HEAD", "--project-config", explicitPolicyPath, "--format=json"},
			{"codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json"},
		} {
			_, stderr, exitCode := runCoachBinary(commandPath, repo, env, args...)
			Expect(exitCode).To(BeNumerically("<=", 1), "stderr: %s", stderr)
		}

		Expect(marker).NotTo(BeAnExistingFile(), "git ran a program the repository's configuration or the environment names")
	})

	It("fails closed, with no report, when GIT_WORK_TREE and GIT_INDEX_FILE point elsewhere, or GIT_DIR is a bare repository seen from a worktree subdirectory", func() {
		repo := commitExplicitPolicyRepo()
		stdout, _, exitCode := runCoachBinary(commandPath, repo, append(os.Environ(), "GIT_WORK_TREE="+GinkgoT().TempDir(), "GIT_INDEX_FILE=/nonexistent/index"), "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).NotTo(Equal(0))
		Expect(stdout).To(BeEmpty())

		bare := repo + ".bare"
		Expect(exec.Command("git", "clone", "-q", "--bare", repo, bare).Run()).To(Succeed())
		DeferCleanup(func() { _ = os.RemoveAll(bare) })
		subdirectory := filepath.Join(repo, "policies")
		stdout, _, exitCode = runCoachBinary(commandPath, subdirectory, append(os.Environ(), "GIT_DIR="+bare), "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).NotTo(Equal(0))
		Expect(stdout).To(BeEmpty())
	})

	It("fails closed, with no report, when GIT_DIR or a .git file points at a different repository", func() {
		repo := commitExplicitPolicyRepo()
		other := commitScannableGoRepo()

		_, _, viaEnv := runCoachBinary(commandPath, repo, append(os.Environ(), "GIT_DIR="+filepath.Join(other, ".git")), "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(viaEnv).NotTo(Equal(0), "a scan read a policy from the working tree of one repository and the history of another")

		Expect(os.RemoveAll(filepath.Join(repo, ".git"))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: "+filepath.Join(other, ".git")+"\n"), 0o644)).To(Succeed())
		stdout, _, viaFile := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(viaFile).NotTo(Equal(0))
		Expect(stdout).To(BeEmpty())
	})
})

var _ = Describe("coach codesignal --suggest-project-config --output: the destination named on the command line", func() {
	suggestRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", "module example.com/m\n\ngo 1.22\n")
		commitFile(repo, "a.go", minimalGoSource)
		return repo
	}

	It("refuses a destination whose parent directory does not exist, naming it", func() {
		repo := suggestRepo()

		_, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--suggest-project-config", "--output", "cfg/new.json")

		Expect(exitCode).To(Equal(2))
		Expect(string(stderr)).To(ContainSubstring(`parent directory \"cfg\" does not exist`))
	})

	It("refuses a FIFO destination without waiting on it", func() {
		repo := suggestRepo()
		Expect(syscall.Mkfifo(filepath.Join(repo, "f.json"), 0o644)).To(Succeed())

		start := time.Now()
		_, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--suggest-project-config", "--output", "f.json")

		Expect(exitCode).To(Equal(2))
		Expect(string(stderr)).To(ContainSubstring("--output target already exists"))
		Expect(time.Since(start)).To(BeNumerically("<", 10*time.Second))
	})

	It("creates a file under a name full of shell metacharacters and a newline, runs nothing, and escapes the name it reports", func() {
		repo := suggestRepo()
		name := "$(touch PWN)\nx.json"

		_, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--suggest-project-config", "--output", name)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(filepath.Join(repo, name)).To(BeARegularFile())
		Expect(filepath.Join(repo, "PWN")).NotTo(BeAnExistingFile())
		Expect(string(stderr)).NotTo(ContainSubstring("\n$(touch"), "the newline in the name reached the terminal raw")
	})
})

var _ = Describe("coach codesignal: an explicit --project-config path raced on disk while the scan runs", func() {
	It("never hangs, panics, prints a report beside a refusal, or reads a file outside the repository", func() {
		const canary = "TOP-SECRET-CANARY-7f3a91"
		repo := commitExplicitPolicyRepo()
		target := filepath.Join(repo, explicitPolicyPath)
		side := GinkgoT().TempDir()
		outside := filepath.Join(side, "outside.json")
		Expect(os.WriteFile(outside, []byte(`{"canary":"`+canary+`","schema_version":"1","roots":["."]}`), 0o644)).To(Succeed())
		regular := filepath.Join(side, "regular.json")
		Expect(os.WriteFile(regular, []byte(singleRootPolicyJSON), 0o644)).To(Succeed())

		stop := make(chan struct{})
		var swapper sync.WaitGroup
		swapper.Add(1)
		go func() {
			defer swapper.Done()
			scratch := target + ".swap"
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				_ = os.Remove(scratch)
				switch i % 5 {
				case 0, 1:
					_ = os.WriteFile(scratch, []byte(singleRootPolicyJSON), 0o644)
				case 2:
					_ = syscall.Mkfifo(scratch, 0o644)
				case 3:
					_ = os.Symlink(outside, scratch)
				case 4:
					_ = os.Symlink(regular, scratch)
				}
				_ = os.Rename(scratch, target)
				if i%17 == 0 {
					_ = os.Remove(target)
				}
			}
		}()
		DeferCleanup(func() { close(stop); swapper.Wait() })

		for i := 0; i < 40; i++ {
			begin := time.Now()
			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")

			Expect(time.Since(begin)).To(BeNumerically("<", 20*time.Second), "a scan hung on the raced entry")
			Expect(exitCode).To(BeElementOf(0, 2), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("panic:"))
			Expect(string(stdout) + string(stderr)).NotTo(ContainSubstring(canary))
			if exitCode == 2 {
				Expect(stdout).To(BeEmpty(), "a report was printed beside a refusal")
			}
		}
	})
})
