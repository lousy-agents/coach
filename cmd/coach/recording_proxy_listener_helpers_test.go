package main

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/gomega"
)

type recordingProxyListener struct {
	addr string
	mu   sync.Mutex
	hits []string
	stop func()
}

func startRecordingProxyListener() *recordingProxyListener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).NotTo(HaveOccurred())

	rec := &recordingProxyListener{addr: ln.Addr().String()}
	var inflight sync.WaitGroup
	acceptDone := make(chan struct{})
	go rec.serveRecordedProxyLines(ln, &inflight, acceptDone)
	rec.stop = func() {
		_ = ln.Close()
		<-acceptDone
		inflight.Wait()
	}
	return rec
}

// serveRecordedProxyLines accepts connections until ln is closed, recording
// each connection's first request line.
func (r *recordingProxyListener) serveRecordedProxyLines(ln net.Listener, inflight *sync.WaitGroup, acceptDone chan struct{}) {
	defer close(acceptDone)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		inflight.Add(1)
		go r.recordRequestLine(conn, inflight)
	}
}

func (r *recordingProxyListener) recordRequestLine(c net.Conn, inflight *sync.WaitGroup) {
	defer inflight.Done()
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	line, _ := bufio.NewReader(c).ReadString('\n')
	r.mu.Lock()
	r.hits = append(r.hits, strings.TrimSpace(line))
	r.mu.Unlock()
}

func (r *recordingProxyListener) proxyURL() string {
	return "http://" + r.addr
}

func (r *recordingProxyListener) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.hits))
	copy(out, r.hits)
	return out
}
