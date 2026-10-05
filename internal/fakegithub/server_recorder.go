package fakegithub

import (
	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// Recorder returns the request recorder for sequence/auth assertions.
func (s *Server) Recorder() *acceptanceharness.Recorder { return s.recorder }
