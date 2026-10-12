package tssidecar

import "bytes"

// boundedWriter retains only the first limit bytes written to it, silently
// discarding the rest, so an untrusted child's stderr cannot grow the
// diagnostic message without bound.
type boundedWriter struct {
	buf   bytes.Buffer
	limit int
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if room := w.limit - w.buf.Len(); room > 0 {
		if room > len(p) {
			room = len(p)
		}
		w.buf.Write(p[:room])
	}
	return len(p), nil
}
