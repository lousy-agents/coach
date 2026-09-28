package main

import (
	"io"
	"net"
	"sync"
	"time"
)

type sigstartNamespaceDialListener40498254 struct {
	acceptDone chan struct {
	}
	inflight sync.
			WaitGroup
	ln net.
		Listener
	rec *namespaceDialListener
}

func (sigRecv *sigstartNamespaceDialListener40498254) call() {
	defer close(sigRecv.acceptDone)
	for {
		conn, err := sigRecv.ln.Accept()
		if err != nil {
			return
		}
		sigRecv.inflight.
			Add(1)
		go func(c net.Conn) {
			defer sigRecv.inflight.Done()
			defer c.Close()
			sigRecv.rec.
				mu.Lock()
			sigRecv.rec.
				hits = append(sigRecv.rec.hits, c.RemoteAddr().String())
			sigRecv.rec.
				mu.Unlock()
			_ = c.SetDeadline(time.Now().Add(2 * time.Second))
			_, _ = io.Copy(io.Discard, c)
		}(conn)
	}
}
