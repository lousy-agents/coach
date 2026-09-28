package main

import (
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func body_projectTsLinuxConfinementAcceptanceTest_21() {
	if runtime.GOOS != "linux" {
		Skip(linuxConfinementElsewhereSkip)
	}
	ensureFileSyscallTracer()
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsLinuxConfinementAcceptanceTest_76() {
	if runtime.GOOS != "linux" {
		Skip(linuxConfinementElsewhereSkip)
	}
	ensureUnshareAvailable()
}

func body_projectTsLinuxConfinementAcceptanceTest_failsTheIdenticalDialInsideUnshareWithADistincti_92() {
	listener, host, port := startNamespaceDialListener()
	DeferCleanup(listener.stop)
	mod := writeFakeCompilerDialModule(host, port)
	hitsBefore := len(listener.snapshot())
	stdout, stderr, err := runFakeCompilerDial(unsharePrefix(), mod)
	Expect(err).To(HaveOccurred(), "removing unshare must fail this When because the inside dial then succeeds; stdout=%s stderr=%s", stdout, stderr)
	msg := strings.ToUpper(stderr + stdout + err.Error())
	if listener.loopbackFallback {
		Expect(msg).To(Or(ContainSubstring("ECONNREFUSED"), ContainSubstring("ENETUNREACH"), ContainSubstring("EHOSTUNREACH")),
			"inside dial distinctive error; ECONNREFUSED is never the sole signal: outside hit is required too. stdout=%s stderr=%s", stdout, stderr)
		Expect(hitsBefore).To(Equal(0), "inside half must not be the only observation")
		stdout2, stderr2, err2 := runFakeCompilerDial(nil, mod)
		Expect(err2).NotTo(HaveOccurred(), "outside hit is required so ECONNREFUSED is not the sole signal; stdout=%s stderr=%s", stdout2, stderr2)
		Expect(listener.snapshot()).NotTo(BeEmpty())
		return
	}
	Expect(msg).To(Or(ContainSubstring("ENETUNREACH"), ContainSubstring("EHOSTUNREACH")),
		"preferred inside error is ENETUNREACH or EHOSTUNREACH, not ECONNREFUSED; stdout=%s stderr=%s", stdout, stderr)
	Expect(listener.snapshot()).To(HaveLen(hitsBefore), "inside namespace must not reach the outside listener")
}

func body_projectTsLinuxConfinementAcceptanceTest_116() {
	if runtime.GOOS != "linux" {
		Skip(linuxConfinementElsewhereSkip)
	}
	ensureUnshareAvailable()
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}
