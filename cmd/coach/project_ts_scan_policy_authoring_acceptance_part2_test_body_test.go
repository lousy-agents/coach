//go:build linux

package main

import "os"

func body_projectTsScanPolicyAuthoringAcceptancePart2Test_51(master *os.File, session *controllingTerminalSession) {
	defer close(session.drained)
	buf := make([]byte, 4096)
	for {
		n, readErr := master.Read(buf)
		if n > 0 {
			session.mu.Lock()
			session.transcript.Write(buf[:n])
			session.mu.Unlock()
		}
		if readErr != nil {
			return
		}
	}
}
