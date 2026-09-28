package main

import (
	"strings"
)

func pathValueFromEnviron(env string) string {
	for _, part := range strings.FieldsFunc(env, func(r rune) bool { return r == '\n' || r == '\x00' }) {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "PATH=") {
			return strings.TrimSpace(strings.TrimPrefix(part, "PATH="))
		}
		if i := strings.Index(part, "PATH="); i >= 0 {
			rest := part[i+5:]
			if j := strings.IndexAny(rest, " \t"); j >= 0 {
				return rest[:j]
			}
			return rest
		}
	}
	if i := strings.Index(env, "PATH="); i >= 0 {
		rest := env[i+5:]
		if j := strings.IndexAny(rest, " \n\t"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	return ""
}

// isDescendantOfProcess reports whether pid's parent chain, read from
// /proc/<pid>/stat, reaches ancestor before hitting PID 1 or a read failure.
func isDescendantOfProcess(pid, ancestor int) bool {
	seen := make(map[int]bool)
	for {
		if pid == ancestor {
			return true
		}
		if pid <= 1 || seen[pid] {
			return false
		}
		seen[pid] = true
		ppid, ok := processParentPID(pid)
		if !ok {
			return false
		}
		pid = ppid
	}
}

func (s *analyzerEnvironSampler) halt() map[int]string {
	close(s.stop)
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[int]string, len(s.byPID))
	for pid, env := range s.byPID {
		out[pid] = env
	}
	return out
}

func newIdleAnalyzerEnvironSampler() *analyzerEnvironSampler {
	return &analyzerEnvironSampler{
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
		byPID: make(map[int]string),
		seen:  make(map[string]struct{}),
	}
}
