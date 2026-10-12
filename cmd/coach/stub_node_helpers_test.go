package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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

	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo %s; exit 0; fi\nif [ \"$1\" = \"-p\" ]; then echo \"$0\"; exit 0; fi\necho %s\n", version, version)
	Expect(os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathExcludingToolchain strips every real node/npm/mise directory, so a spec
// on this PATH has no global-mise candidate however the host is configured.
// The package managers are stripped alongside them because
// checkPackageManager probes whichever npm/pnpm/bun/yarn the child can
// resolve: leaving the host's own installation reachable would make a
// package-manager classification depend on which manager this machine happens
// to have.
func pathExcludingToolchain() string {
	return pathExcludingExecutables("node", "npm", "mise", "pnpm", "bun", "yarn")
}

func pathWithStubNode(version string) string {
	return writeStubNodeScript(version) + string(os.PathListSeparator) + pathExcludingToolchain()
}

// pathWithoutNode names pathExcludingToolchain from the perspective of the
// node_missing specs: with no node reachable, checkNodeReadiness reports
// node_missing regardless of the host's actual Node installation.
func pathWithoutNode() string {
	return pathExcludingToolchain()
}

// requireStubNodeVersion is the belt-and-suspenders probe mirroring
// node_absent_acceptance_test.go's pattern: it proves path's stub node is
// genuinely the one that would be resolved and reports exactly version, so
// a deterministic result cannot be a false green caused by some other
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
// node_missing result cannot be a false green.
func requireNodeUnreachable(path string) {
	probe := exec.Command("sh", "-c", "command -v node || command -v npm")
	probe.Env = []string{"PATH=" + path}
	Expect(probe.Run()).To(HaveOccurred(), "expected neither node nor npm to be found on %q", path)
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
	return writeHangingNodeScript() + string(os.PathListSeparator) + pathExcludingToolchain()
}

// writeFailingNodeScript writes an executable `node` script that exits
// non-zero on any invocation without printing a parsable version, returning
// the directory containing it. This drives detectHostNodeMajor's
// exitErr != nil branch specifically, distinct from a timeout (hangs, never
// exits) or an unparsable-but-successful probe (exits 0 with junk output).
func writeFailingNodeScript() string {
	dir, err := os.MkdirTemp("", "coach-acceptance-failnode-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := "#!/bin/sh\nexit 3\n"
	Expect(os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathWithFailingNode returns a PATH whose first entry is a stub `node`
// that exits non-zero on `--version` without printing output, with every
// directory containing a real node/npm/mise executable removed so the stub
// is the only "node" the child process can resolve.
func pathWithFailingNode() string {
	return writeFailingNodeScript() + string(os.PathListSeparator) + pathExcludingToolchain()
}

// writeUnstartableNodeScript writes an executable file named `node` whose
// shebang names an interpreter that does not exist, returning the directory
// containing it. exec.Cmd.Start() resolves the name via LookPath (it is
// executable, so LookPath succeeds) but the subsequent fork/exec fails,
// distinct from writeFailingNodeScript's case (the process starts and exits
// non-zero) and driving detectHostNodeMajor's cmd.Start() failure path
// specifically.
func writeUnstartableNodeScript() string {
	dir, err := os.MkdirTemp("", "coach-acceptance-unstartnode-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := "#!/nonexistent/interpreter\n"
	Expect(os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathWithUnstartableNode returns a PATH whose first entry is a stub `node`
// that resolves via LookPath but fails to start (a shebang naming a missing
// interpreter), with every directory containing a real node/npm/mise
// executable removed so the stub is the only "node" the child process can
// resolve.
func pathWithUnstartableNode() string {
	return writeUnstartableNodeScript() + string(os.PathListSeparator) + pathExcludingToolchain()
}

// writeOversizedUnparsableNodeScript writes an executable `node` script
// whose `--version` output is a non-parsable blob at maxNodeVersionProbeOutput
// (4 KiB) -- the largest detectHostNodeMajor's own probe budget allows
// without erroring -- returning the directory containing it. This drives
// nodeUnverifiableDetail's rawVersion-embedding branch with the widest input
// it can actually receive, rather than a short literal like "weird-build-2024".
func writeOversizedUnparsableNodeScript() string {
	dir, err := os.MkdirTemp("", "coach-acceptance-bignode-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := "#!/bin/sh\nhead -c 4096 </dev/zero | tr '\\0' x\n"
	Expect(os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathWithOversizedUnparsableNode returns a PATH whose first entry is a stub
// `node` printing a 4 KiB unparsable blob on any invocation (including
// `--version`), with every directory containing a real node/npm/mise
// executable removed so the stub is the only "node" the child process can
// resolve.
func pathWithOversizedUnparsableNode() string {
	return writeOversizedUnparsableNodeScript() + string(os.PathListSeparator) + pathExcludingToolchain()
}
