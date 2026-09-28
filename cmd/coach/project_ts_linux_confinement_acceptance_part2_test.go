package main

import (
	"fmt"
	"io"
	"net"

	"os/exec"

	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func probeLeaksOn(allow linuxAllowlist, rec straceRecord, repo string) []string {
	if _, probe := linuxProbeSyscalls[rec.Syscall]; !probe {
		return nil
	}
	if !straceSucceeded(rec) {
		return nil
	}
	var leaks []string
	for _, p := range rec.Paths {
		if linuxProbeAllowed(allow, rec.Syscall, p) {
			continue
		}
		failIfRepoOpenOrListing(rec, repo, p)
		leaks = append(leaks, fmt.Sprintf("%s (path %s)", rec.Raw, p))
	}
	return leaks
}

func startNamespaceDialListener() (*namespaceDialListener, string, int) {
	rec := &namespaceDialListener{}
	host, ok := nonLoopbackIPv4()
	addr := host + ":0"
	if !ok {
		addr = "127.0.0.1:0"
		rec.loopbackFallback = true
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil && !rec.loopbackFallback {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		rec.loopbackFallback = true
	}
	Expect(err).NotTo(HaveOccurred())
	tcpAddr := ln.Addr().(*net.TCPAddr)
	var inflight sync.WaitGroup
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			inflight.Add(1)
			go func(c net.Conn) {
				defer inflight.Done()
				defer c.Close()
				rec.mu.Lock()
				rec.hits = append(rec.hits, c.RemoteAddr().String())
				rec.mu.Unlock()
				_ = c.SetDeadline(time.Now().Add(2 * time.Second))
				_, _ = io.Copy(io.Discard, c)
			}(conn)
		}
	}()
	rec.stop = func() {
		_ = ln.Close()
		<-acceptDone
		inflight.Wait()
	}
	return rec, tcpAddr.IP.String(), tcpAddr.Port
}

func ensureFileSyscallTracer() {
	if _, err := exec.LookPath("strace"); err == nil {
		return
	}
	_ = exec.Command("sudo", "apt-get", "update").Run()
	if err := exec.Command("sudo", "apt-get", "install", "-y", "strace").Run(); err != nil {
		Fail(fileSyscallTracerUnavailable)
	}
	if _, err := exec.LookPath("strace"); err != nil {
		Fail(fileSyscallTracerUnavailable)
	}
}
