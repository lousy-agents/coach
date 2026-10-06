package tssetup

import (
	"bytes"
	"sync"
)

// boundedOutputSink is an io.Writer that keeps at most limit bytes,
// silently discarding anything past that bound. Execute assigns the
// same *boundedOutputSink to both cmd.Stdout and cmd.Stderr; os/exec
// detects that the two writers are identical (via interfaceEqual, not
// identical-*os.File) and collapses them onto a single pipe read by a
// single copier goroutine, so in practice Write is never called
// concurrently here. The mutex guards Bytes() against a future change that
// gives Stdout and Stderr distinct writers, which would restore concurrent
// Writes.
type boundedOutputSink struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	limit int
}

func (s *boundedOutputSink) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.buf.Bytes()...)
}

func (s *boundedOutputSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if remaining := s.limit - s.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			s.buf.Write(p[:remaining])
		} else {
			s.buf.Write(p)
		}
	}
	return len(p), nil
}
