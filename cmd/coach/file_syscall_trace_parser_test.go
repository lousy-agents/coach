package main

import (
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("file-syscall trace parser", func() {
	When("the trace contains a version probe, a git execve, and an analyzer execve", func() {
		execPath := "/usr/bin/node"
		var recs []straceRecord
		var id straceRecord

		BeforeEach(func() {
			lines := []string{
				`1 execve("/tmp/coach", ["/tmp/coach", "codesignal", "--baseline"], 0x1) = 0`,
				`4 execve("/usr/bin/git", ["git", "ls-tree"], 0x1) = 0`,
				`2 execve("/usr/bin/node", ["/usr/bin/node", "--version"], 0x1) = 0`,
				`3 execve("/usr/bin/node", ["/usr/bin/node", "/tmp/coach-ts-analyzer-abc/coach-ts-project-sidecar", "--compiler-module=/compiler", "--native-package=/native"], 0x1) = 0`,
				`3 openat(AT_FDCWD, "/compiler/package.json", O_RDONLY) = 3`,
				`3 openat(AT_FDCWD, "/etc/passwd", O_RDONLY) = 4`,
				`3 openat(AT_FDCWD, "/home/runner/secret", O_RDONLY) = 5`,
				`3 openat(AT_FDCWD, "/tmp/decoy-typeroots", O_RDONLY) = -1 ENOENT (No such file or directory)`,
			}
			recs = parseStraceLines(lines)
			var ok bool
			id, ok = findAnalyzerIdentity(recs, execPath)
			Expect(ok).To(BeTrue())
		})

		It("identifies the analyzer execve that carries both argv flags, not the version probe", func() {
			Expect(id.PID).To(Equal(3))
			Expect(id.Pathname).To(Equal(execPath))
			Expect(argvHasPrefix(id.Argv, compilerModuleArgPrefix)).To(BeTrue())
			Expect(argvHasPrefix(id.Argv, nativePackageArgPrefix)).To(BeTrue())
		})

		When("the frozen allowlist is built from those identity directories", func() {
			var allow linuxAllowlist

			BeforeEach(func() {
				allow = frozenLinuxAllowlist("/usr/bin", "/compiler", "/native", "/tmp/coach-ts-analyzer-abc", 3)
			})

			It("does not contain the typeRoots decoy or HOME", func() {
				Expect(linuxPathAllowed(allow, "/tmp/decoy-typeroots")).To(BeFalse())
				Expect(linuxPathAllowed(allow, "/home/runner")).To(BeFalse())
			})

			It("allows the compiler package, /etc/passwd, and OpenSSL config", func() {
				Expect(linuxPathAllowed(allow, "/compiler/package.json")).To(BeTrue())
				Expect(linuxPathAllowed(allow, "/etc/passwd")).To(BeTrue())
				Expect(linuxPathAllowed(allow, "/etc/ssl/openssl.cnf")).To(BeTrue())
			})

			It("allows Node kernel identity probes and metadata-only /tmp but not /tmp children", func() {
				Expect(linuxPathAllowed(allow, "/proc/meminfo")).To(BeTrue())
				Expect(linuxPathAllowed(allow, "/proc/version_signature")).To(BeTrue())
				Expect(linuxPathAllowed(allow, "/proc/sys/vm/overcommit_memory")).To(BeTrue())
				Expect(linuxPathAllowed(allow, "/sys/fs/cgroup/memory.max")).To(BeTrue())
				Expect(linuxPathAllowed(allow, "/sys/devices/system/cpu/online")).To(BeTrue())
				Expect(linuxPathAllowed(allow, "/tmp")).To(BeFalse())
				Expect(linuxProbeAllowed(allow, "newfstatat", "/tmp")).To(BeTrue())
				Expect(linuxProbeAllowed(allow, "openat", "/tmp")).To(BeFalse())
				Expect(linuxPathAllowed(allow, "/tmp/secret")).To(BeFalse())
			})

			It("rejects a path under HOME", func() {
				Expect(linuxPathAllowed(allow, "/home/runner/secret")).To(BeFalse())
			})
		})
	})

	When("the compiler path sits under a repository directory", func() {
		var allow linuxAllowlist

		BeforeEach(func() {
			allow = frozenLinuxAllowlist(
				"/usr/bin",
				"/tmp/repo-1/node_modules/typescript",
				"/tmp/repo-1/node_modules/@typescript/native-preview",
				"/tmp/coach-ts-analyzer-abc",
				3,
			)
		})

		It("rejects openat of repository source and a sibling node_modules path", func() {
			Expect(linuxPathAllowed(allow, "/tmp/repo-1")).To(BeFalse())
			Expect(linuxPathAllowed(allow, "/tmp/repo-1/node_modules")).To(BeFalse())
			Expect(linuxProbeAllowed(allow, "openat", "/tmp/repo-1/pkg/handlers/h.ts")).To(BeFalse())
			Expect(linuxProbeAllowed(allow, "openat", "/tmp/repo-1/node_modules/other/index.js")).To(BeFalse())
		})

		It("tolerates a successful newfstatat on the repository root", func() {
			Expect(linuxPathAllowed(allow, "/tmp/repo-1")).To(BeFalse(), "repository root must not be an open or listing prefix")
			Expect(linuxProbeAllowed(allow, "newfstatat", "/tmp/repo-1")).To(BeTrue())
		})
	})

	When("unshare must keep the resolved Node first on PATH", func() {
		It("prefixes PATH with the Node directory and passes PATH, HOME, and TMPDIR through env", func() {
			got := unsharePathEnv("/home/runner/.local/share/mise/installs/node/24.0.0/bin/node", "/usr/local/bin:/usr/bin")
			Expect(got).To(HavePrefix("/home/runner/.local/share/mise/installs/node/24.0.0/bin:"))
			Expect(got).To(HaveSuffix("/usr/local/bin:/usr/bin"))
			wrapper := unshareEnvWrapper(got, "/home/runner", "/tmp")
			Expect(wrapper[0]).To(Equal("env"))
			Expect(wrapper).To(ContainElement("PATH=" + got))
			Expect(wrapper).To(ContainElement("HOME=/home/runner"))
			Expect(wrapper).To(ContainElement("TMPDIR=/tmp"))
			Expect(wrapper).NotTo(ContainElement("GIT_CONFIG_COUNT=1"))
		})
	})

	When("snapshot git sanitizes the parent environ and only forwards HOME", func() {
		It("makes git honor safe.directory from the unshare HOME gitconfig with GIT_CONFIG_NOSYSTEM set", func() {
			home := writeUnshareGitHome()
			cmd := exec.Command("git", "config", "--global", "--get", "safe.directory")
			cmd.Env = []string{
				"PATH=" + os.Getenv("PATH"),
				"HOME=" + home,
				"GIT_TERMINAL_PROMPT=0",
				"GIT_CONFIG_NOSYSTEM=1",
				"GIT_NO_LAZY_FETCH=1",
			}
			out, err := cmd.Output()
			Expect(err).NotTo(HaveOccurred(), "git must read safe.directory from HOME gitconfig; stderr would mean the unshare HOME seam is wrong")
			Expect(strings.TrimSpace(string(out))).To(Equal("*"))
		})
	})

	When("strace splits the analyzer execve across unfinished and resumed lines", func() {
		It("identifies ExecPath plus both argv flags from the unfinished half", func() {
			execPath := "/usr/bin/node"
			lines := []string{
				`3 execve("/usr/bin/node", ["/usr/bin/node", "/tmp/coach-ts-analyzer-abc/coach-ts-project-sidecar", "--compiler-module=/compiler", "--native-package=/native"] <unfinished ...>`,
				`3 <... execve resumed> ) = 0`,
			}
			recs := parseStraceLines(lines)
			id, ok := findAnalyzerIdentity(recs, execPath)
			Expect(ok).To(BeTrue())
			Expect(id.PID).To(Equal(3))
			Expect(argvHasPrefix(id.Argv, compilerModuleArgPrefix)).To(BeTrue())
			Expect(argvHasPrefix(id.Argv, nativePackageArgPrefix)).To(BeTrue())
		})
	})

	When("strace -f logs a coach OS thread that stats git", func() {
		It("does not treat that LookPath as an analyzer-subtree allowlist miss", func() {
			execPath := "/home/runner/.local/share/mise/installs/node/24.0.0/bin/node"
			decoy := "/tmp/decoy-typeroots"
			lines := []string{
				`1 execve("/tmp/coach", ["/tmp/coach", "codesignal", "--baseline"], 0x1) = 0`,
				`1 clone(child_stack=NULL, flags=CLONE_VM|CLONE_FS|CLONE_FILES|CLONE_SIGHAND|CLONE_THREAD, tls=0x1) = 10`,
				`10 newfstatat(AT_FDCWD</tmp/coach-acceptance-repo-1>, "/usr/bin/git", {st_mode=S_IFREG|0755, st_size=1}, 0) = 0`,
				`2 execve("` + execPath + `", ["` + execPath + `", "--version"], 0x1) = 0`,
				`1 clone(child_stack=NULL, flags=CLONE_VM|CLONE_VFORK, tls=0x1) = 3`,
				`3 execve("` + execPath + `", ["` + execPath + `", "/tmp/coach-ts-analyzer-abc/coach-ts-project-sidecar", "--compiler-module=/compiler", "--native-package=/native"], 0x1) = 0`,
				`3 clone(child_stack=NULL, flags=CLONE_VM|CLONE_FS|CLONE_FILES|CLONE_SIGHAND|CLONE_THREAD, tls=0x1) = 11`,
				`11 openat(AT_FDCWD</tmp/coach-ts-analyzer-abc>, "/compiler/package.json", O_RDONLY) = 3`,
				`11 openat(AT_FDCWD</tmp/coach-ts-analyzer-abc>, "/etc/passwd", O_RDONLY) = 4`,
			}
			recs := parseStraceLines(lines)
			id, ok := findAnalyzerIdentity(recs, execPath)
			Expect(ok).To(BeTrue())
			Expect(id.PID).To(Equal(3))
			judgeConfinedAnalyzerSubtree(recs, id, execPath, decoy, "", false)
		})
	})
})
