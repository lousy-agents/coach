package main

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"time"
)

type sigstartRecordingProxyListener39957725 struct {
	acceptDone chan struct {
	}
	inflight *sync.WaitGroup
	ln       net.
			Listener
	rec *recordingProxyListener
}

func (sigRecv *sigstartRecordingProxyListener39957725) call() {
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
			_ = c.SetDeadline(time.Now().Add(2 * time.Second))
			line, _ := bufio.NewReader(c).ReadString('\n')
			sigRecv.rec.
				mu.Lock()
			sigRecv.rec.
				hits = append(sigRecv.rec.hits, strings.TrimSpace(line))
			sigRecv.rec.
				mu.Unlock()
		}(conn)
	}
}

type siganalyzerChildPIDsFromPSS10 struct {
	args       string
	candidates *[]int
	pid        int
}

func (sigRecv *siganalyzerChildPIDsFromPSS10) call() {

	if strings.Contains(sigRecv.args, analyzerChildArgMarker) {
		*sigRecv.candidates = append(*sigRecv.candidates, sigRecv.pid)
	}
}
