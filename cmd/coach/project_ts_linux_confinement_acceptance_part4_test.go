package main

import (
	"errors"
	"fmt"

	"net"
	"os"
	"os/exec"
	"path/filepath"

	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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

func decoyHitsOn(rec straceRecord, decoy string) []string {
	var hits []string
	for _, p := range rec.Paths {
		if pathHasPrefix(p, decoy) {
			hits = append(hits, rec.Raw)
		}
	}
	return hits
}

func ifaceNonLoopbackIPv4(iface net.Interface) (string, bool) {
	if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
		return "", false
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", false
	}
	return firstNonLoopbackIPv4(addrs)
}

func failIfRepoOpenOrListing(rec straceRecord, repo, path string) {
	_, openOrListing := linuxOpenOrListingSyscalls[rec.Syscall]
	if repo != "" && openOrListing && pathHasPrefix(path, repo) {
		Fail(fmt.Sprintf("successful open or listing under the fixture repository root; do not add an allowlist entry: %s (path %s)", rec.Raw, path))
	}
}

func runCoachBaselineUnderUnshare(repo string) (stdout, stderr []byte, exitCode int) {
	prefix := append(append([]string{}, unsharePrefix()...), unshareEnvWrapper(unsharePathEnv(probedNodeExecPath(), os.Getenv("PATH")), writeUnshareGitHome(), os.Getenv("TMPDIR"))...)
	args := append(append([]string{}, prefix...), commandPath, "codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = repo
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = []byte(outBuf.String())
	stderr = []byte(errBuf.String())
	if err == nil {
		return stdout, stderr, 0
	}
	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return stdout, stderr, exitErr.ExitCode()
}
