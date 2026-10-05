package main

import (
	"io"
	"net"
	"sync"
	"time"

	. "github.com/onsi/gomega"
)

type namespaceDialListener struct {
	hits             []string
	mu               sync.Mutex
	stop             func()
	loopbackFallback bool
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
	go rec.serveDialHits(ln, &inflight, acceptDone)
	rec.stop = func() {
		_ = ln.Close()
		<-acceptDone
		inflight.Wait()
	}
	return rec, tcpAddr.IP.String(), tcpAddr.Port
}

// serveDialHits accepts connections until ln is closed, recording each
// dialer's remote address.
func (l *namespaceDialListener) serveDialHits(ln net.Listener, inflight *sync.WaitGroup, acceptDone chan struct{}) {
	defer close(acceptDone)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		inflight.Add(1)
		go l.recordDialHit(conn, inflight)
	}
}

func (l *namespaceDialListener) recordDialHit(c net.Conn, inflight *sync.WaitGroup) {
	defer inflight.Done()
	defer c.Close()
	l.mu.Lock()
	l.hits = append(l.hits, c.RemoteAddr().String())
	l.mu.Unlock()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = io.Copy(io.Discard, c)
}

func (l *namespaceDialListener) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.hits))
	copy(out, l.hits)
	return out
}
