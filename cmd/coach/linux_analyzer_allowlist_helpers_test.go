package main

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"
)

type ancestorAcc struct {
	seen map[string]struct{}
	out  []string
}

type linuxAllowlist struct {
	exact     []string
	prefixes  []string
	ancestors []string
}

func linuxAnalyzerAllowlist(recs []straceRecord, id straceRecord, execPath string) (linuxAllowlist, map[int]struct{}) {
	compilerDir := argvValue(id.Argv, compilerModuleArgPrefix)
	nativeDir := argvValue(id.Argv, nativePackageArgPrefix)
	shim := ""
	if len(id.Argv) > 1 {
		shim = id.Argv[1]
	}
	Expect(shim).NotTo(BeEmpty(), "analyzer shim path missing from identity argv")
	allow := frozenLinuxAllowlist(filepath.Dir(execPath), compilerDir, nativeDir, filepath.Dir(shim), id.PID)
	subtree := analyzerSubtreePIDs(recs, id.PID)
	for pid := range subtree {
		allow.prefixes = append(allow.prefixes, fmt.Sprintf("/proc/%d", pid))
	}
	return allow, subtree
}

func assertLinuxAllowlistNotOpenListing(allow linuxAllowlist, decoy, repo string) {
	Expect(linuxPathAllowed(allow, decoy)).To(BeFalse(), "decoy and HOME are asserted not in the frozen allowlist before judgment")
	if home := os.Getenv("HOME"); home != "" {
		Expect(linuxPathAllowed(allow, home)).To(BeFalse(), "decoy and HOME are asserted not in the frozen allowlist before judgment")
	}
	if repo == "" {
		return
	}
	Expect(linuxPathAllowed(allow, repo)).To(BeFalse(), "fixture repository root is not allowed for opens and listings")
	Expect(linuxPathAllowed(allow, filepath.Join(repo, "node_modules"))).To(BeFalse(), "fixture repository node_modules is not allowed for opens and listings")
}

func exactAncestorComponents(dirs ...string) []string {
	acc := &ancestorAcc{seen: map[string]struct{}{"/tmp": {}}, out: []string{"/tmp"}}
	for _, dir := range dirs {
		acc.addChain(filepath.Clean(dir))
	}
	return acc.out
}

func (a *ancestorAcc) addChain(d string) {
	for {
		parent := filepath.Dir(d)
		if parent == d || parent == "." || parent == string(os.PathSeparator) {
			return
		}
		a.remember(parent)
		d = parent
	}
}

func (a *ancestorAcc) remember(path string) {
	path = filepath.Clean(path)
	if path == "." || path == string(os.PathSeparator) {
		return
	}
	if _, ok := a.seen[path]; ok {
		return
	}
	a.seen[path] = struct{}{}
	a.out = append(a.out, path)
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
