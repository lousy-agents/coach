package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Frozen engineer-facing file-syscall list (D-LNX-01). Do not use %file.
var linuxFileSyscalls = []string{
	"open", "openat", "openat2", "creat",
	"getdents", "getdents64",
	"stat", "lstat", "newfstatat", "statx",
	"access", "faccessat", "faccessat2",
	"readlink", "readlinkat",
	"execve", "execveat",
	"unlink", "unlinkat", "rename", "renameat", "renameat2",
	"mkdir", "mkdirat",
	"chmod", "fchmod", "fchmodat",
}

var linuxMutationSyscalls = map[string]struct{}{
	"unlink": {}, "unlinkat": {},
	"rename": {}, "renameat": {}, "renameat2": {},
	"mkdir": {}, "mkdirat": {},
	"chmod": {}, "fchmod": {}, "fchmodat": {},
}

var linuxProbeSyscalls = map[string]struct{}{
	"open": {}, "openat": {}, "openat2": {}, "creat": {},
	"getdents": {}, "getdents64": {},
	"stat": {}, "lstat": {}, "newfstatat": {}, "statx": {},
	"access": {}, "faccessat": {}, "faccessat2": {},
	"readlink": {}, "readlinkat": {},
}

var linuxOpenOrListingSyscalls = map[string]struct{}{
	"open": {}, "openat": {}, "openat2": {}, "creat": {},
	"getdents": {}, "getdents64": {},
}

var linuxMetadataSyscalls = map[string]struct{}{
	"stat": {}, "lstat": {}, "newfstatat": {}, "statx": {},
	"access":   {},
	"readlink": {}, "readlinkat": {},
}

const (
	linuxConfinementElsewhereSkip = "linux file-syscall and network-namespace controls run only on linux; they Fail, not Skip, in the ts-project-backend CI job"
	fileSyscallTracerUnavailable  = "file-syscall tracer unavailable; file-syscall control must Fail, not Skip, inside Label(\"ts-project-backend\")"
	unshareUnavailable            = "unshare -rn and privileged unshare -n both unavailable; network-namespace control must Fail, not Skip, inside Label(\"ts-project-backend\")"
	unconfinedArgvHook            = "--coach-test-unconfined"
	compilerModuleArgPrefix       = "--compiler-module="
	nativePackageArgPrefix        = "--native-package="
)

var (
	stracePIDLine   = regexp.MustCompile(`^(\d+)\s+(.*)$`)
	straceFdPath    = regexp.MustCompile(`<(/[^>]*)>`)
	straceQuoted    = regexp.MustCompile(`"((?:\\.|[^"\\])*)"`)
	straceResultAt  = regexp.MustCompile(`\)\s*=\s*(\S+)`)
	straceSyscallAt = regexp.MustCompile(`^(?:<\.\.\.\s+)?([a-z0-9_]+)`)
)

type linuxAllowlist struct {
	exact     []string
	prefixes  []string
	ancestors []string
}

type straceRecord struct {
	PID      int
	Syscall  string
	Raw      string
	Result   string
	Paths    []string
	Argv     []string
	Pathname string
}

func frozenLinuxAllowlist(runtimeDir, compilerDir, nativeDir, analyzerDir string, analyzerRootPID int) linuxAllowlist {
	runtimeDir = filepath.Clean(runtimeDir)
	compilerDir = filepath.Clean(compilerDir)
	nativeDir = filepath.Clean(nativeDir)
	analyzerDir = filepath.Clean(analyzerDir)
	return linuxAllowlist{
		exact: []string{
			"/etc/ld.so.cache",
			"/dev/null",
			"/dev/zero",
			"/dev/urandom",
			"/etc/nsswitch.conf",
			"/etc/passwd",
			"/etc/group",
			"/etc/hosts",
			"/proc/meminfo",
			"/proc/version",
			"/proc/version_signature",
			"/proc/cpuinfo",
			"/proc/stat",
			"/proc/uptime",
			"/proc/loadavg",
		},
		prefixes: []string{
			runtimeDir,
			compilerDir,
			nativeDir,
			analyzerDir,
			"/lib",
			"/lib64",
			"/usr/lib",
			"/usr/lib64",
			"/usr/share/locale",
			"/usr/lib/locale",
			"/usr/share/i18n",
			"/usr/share/zoneinfo",
			"/etc/ssl",
			"/proc/self",
			"/proc/sys",
			fmt.Sprintf("/proc/%d", analyzerRootPID),
			"/sys/fs/cgroup",
			"/sys/devices/system/cpu",
			"/sys/kernel/mm",
		},
		ancestors: exactAncestorComponents(runtimeDir, compilerDir, nativeDir, analyzerDir),
	}
}

func exactAncestorComponents(dirs ...string) []string {
	seen := map[string]struct{}{"/tmp": {}}
	out := []string{"/tmp"}
	add := func(path string) {
		path = filepath.Clean(path)
		if path == "." || path == string(os.PathSeparator) {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	for _, dir := range dirs {
		d := filepath.Clean(dir)
		for {
			parent := filepath.Dir(d)
			if parent == d || parent == "." || parent == string(os.PathSeparator) {
				break
			}
			add(parent)
			d = parent
		}
	}
	return out
}

func linuxPathAllowed(allow linuxAllowlist, path string) bool {
	clean := filepath.Clean(path)
	for _, exact := range allow.exact {
		if clean == filepath.Clean(exact) {
			return true
		}
	}
	for _, prefix := range allow.prefixes {
		p := filepath.Clean(prefix)
		if clean == p || strings.HasPrefix(clean, p+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func linuxAncestorExact(allow linuxAllowlist, path string) bool {
	clean := filepath.Clean(path)
	for _, ancestor := range allow.ancestors {
		if clean == filepath.Clean(ancestor) {
			return true
		}
	}
	return false
}

func linuxProbeAllowed(allow linuxAllowlist, syscall, path string) bool {
	if linuxPathAllowed(allow, path) {
		return true
	}
	if _, meta := linuxMetadataSyscalls[syscall]; meta && linuxAncestorExact(allow, path) {
		return true
	}
	return false
}

func parseStraceFile(path string) []straceRecord {
	f, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred(), "strace log %s", path)
	defer f.Close()
	var recs []straceRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		if rec, ok := parseStraceLine(sc.Text()); ok {
			recs = append(recs, rec)
		}
	}
	Expect(sc.Err()).NotTo(HaveOccurred())
	return recs
}

func parseStraceLines(lines []string) []straceRecord {
	var recs []straceRecord
	for _, line := range lines {
		rec, ok := parseStraceLine(line)
		Expect(ok).To(BeTrue(), line)
		recs = append(recs, rec)
	}
	return recs
}

func parseStraceLine(line string) (straceRecord, bool) {
	line = strings.TrimSpace(line)
	m := stracePIDLine.FindStringSubmatch(line)
	if m == nil {
		return straceRecord{}, false
	}
	pid, err := strconv.Atoi(m[1])
	if err != nil {
		return straceRecord{}, false
	}
	rest := m[2]
	sys := straceSyscallAt.FindStringSubmatch(rest)
	if sys == nil {
		return straceRecord{}, false
	}
	name := sys[1]
	if strings.HasPrefix(name, "...") {
		return straceRecord{}, false
	}
	if strings.Contains(rest, "unfinished") && name != "execve" && name != "execveat" {
		return straceRecord{}, false
	}
	rec := straceRecord{PID: pid, Syscall: name, Raw: line, Paths: stracePaths(rest)}
	if rm := straceResultAt.FindStringSubmatch(rest); rm != nil {
		rec.Result = rm[1]
	}
	if name == "execve" || name == "execveat" {
		quoted := unescapeStraceQuoted(rest)
		if len(quoted) > 0 {
			rec.Pathname = quoted[0]
		}
		rec.Argv = execveArgv(rest)
	}
	return rec, true
}

func stracePaths(rest string) []string {
	seen := map[string]struct{}{}
	var paths []string
	add := func(p string) {
		p = unescapeC(p)
		if p == "" || !strings.HasPrefix(p, "/") {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	for _, q := range straceQuoted.FindAllStringSubmatch(rest, -1) {
		add(q[1])
	}
	for _, q := range straceFdPath.FindAllStringSubmatch(rest, -1) {
		add(q[1])
	}
	return paths
}

func execveArgv(rest string) []string {
	start := strings.Index(rest, "[")
	end := strings.Index(rest, "]")
	if start < 0 || end <= start {
		return unescapeStraceQuoted(rest)
	}
	return unescapeStraceQuoted(rest[start : end+1])
}

func unescapeStraceQuoted(s string) []string {
	var out []string
	for _, q := range straceQuoted.FindAllStringSubmatch(s, -1) {
		out = append(out, unescapeC(q[1]))
	}
	return out
}

func unescapeC(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '"', '\\':
				b.WriteByte(s[i])
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func straceSucceeded(rec straceRecord) bool {
	if rec.Result == "" || strings.HasPrefix(rec.Result, "-") {
		return false
	}
	if strings.Contains(rec.Raw, "ENOENT") || strings.Contains(rec.Raw, "ENOTDIR") {
		return false
	}
	return true
}

func findAnalyzerIdentity(recs []straceRecord, execPath string) (straceRecord, bool) {
	for _, rec := range recs {
		if rec.Syscall != "execve" && rec.Syscall != "execveat" {
			continue
		}
		if rec.Pathname != execPath {
			continue
		}
		if !argvHasPrefix(rec.Argv, compilerModuleArgPrefix) || !argvHasPrefix(rec.Argv, nativePackageArgPrefix) {
			continue
		}
		return rec, true
	}
	return straceRecord{}, false
}

func argvHasPrefix(argv []string, prefix string) bool {
	for _, a := range argv {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

func argvValue(argv []string, prefix string) string {
	for _, a := range argv {
		if strings.HasPrefix(a, prefix) {
			return strings.TrimPrefix(a, prefix)
		}
	}
	return ""
}

func linuxStraceTraceExpr() string {
	return strings.Join(append(append([]string{}, linuxFileSyscalls...), "clone", "clone3", "fork", "vfork"), ",")
}

func cloneChildPID(rec straceRecord) (int, bool) {
	switch rec.Syscall {
	case "clone", "clone3", "fork", "vfork":
	default:
		return 0, false
	}
	if !straceSucceeded(rec) {
		return 0, false
	}
	n, err := strconv.Atoi(rec.Result)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func analyzerSubtreePIDs(recs []straceRecord, rootPID int) map[int]struct{} {
	children := map[int][]int{}
	for _, rec := range recs {
		child, ok := cloneChildPID(rec)
		if !ok {
			continue
		}
		children[rec.PID] = append(children[rec.PID], child)
	}
	out := map[int]struct{}{rootPID: {}}
	stack := []int{rootPID}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, c := range children[p] {
			if _, seen := out[c]; seen {
				continue
			}
			out[c] = struct{}{}
			stack = append(stack, c)
		}
	}
	return out
}

func unsharePathEnv(nodeExecPath, ambientPATH string) string {
	dir := filepath.Dir(nodeExecPath)
	if ambientPATH == "" {
		return dir
	}
	return dir + string(os.PathListSeparator) + ambientPATH
}

func unshareEnvWrapper(pathEnv, home, tmpdir string) []string {
	args := []string{"env", "PATH=" + pathEnv, "HOME=" + home}
	if tmpdir != "" {
		args = append(args, "TMPDIR="+tmpdir)
	}
	return args
}

func repositoryRoot() string {
	_, file, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

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
