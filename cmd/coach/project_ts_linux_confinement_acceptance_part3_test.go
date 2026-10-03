package main

import (
	"fmt"

	"net"

	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func nonLoopbackIPv4() (string, bool) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", false
	}
	for _, iface := range ifaces {
		if ip, ok := ifaceNonLoopbackIPv4(iface); ok {
			return ip, true
		}
	}
	return "", false
}

func firstNonLoopbackIPv4(addrs []net.Addr) (string, bool) {
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil {
			continue
		}
		return ip.String(), true
	}
	return "", false
}

func ensureUnshareAvailable() {
	if _, err := exec.LookPath("unshare"); err != nil {
		Fail(unshareUnavailable)
	}
	if exec.Command("unshare", "-rn", "--", "true").Run() == nil {
		namespaceUnsharePrefix = []string{"unshare", "-rn", "--"}
		return
	}
	if exec.Command("sudo", "unshare", "-n", "--", "true").Run() == nil {
		namespaceUnsharePrefix = []string{"sudo", "unshare", "-n", "--"}
		return
	}
	Fail(unshareUnavailable)
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
