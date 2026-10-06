package main

import (
	"strconv"
	"strings"
	"sync"
)

type analyzerEnvironSampler struct {
	stop  chan struct{}
	done  chan struct{}
	mu    sync.Mutex
	byPID map[int]string
	seen  map[string]struct{}
}

func (s *analyzerEnvironSampler) invocations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.seen)
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

// record counts a cmdline-matched analyzer child even when its environ
// cannot be read. The kernel returns an empty environ for a task that has
// already torn down its address space; treating that as no observation
// (see readProcessEnviron) is right for PATH assertions and wrong for
// invocation counts, because a warm second spawn is often only visible
// during teardown. A starttime-keyed sample and a later pid-only sample
// of the same PID are one invocation (stat can fail at teardown); two
// starttimes for the same PID are two sequential children.
func (s *analyzerEnvironSampler) record(pid int, startTime, env string, envOK bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pidKey := strconv.Itoa(pid)
	if startTime != "" {
		delete(s.seen, pidKey)
		s.seen[pidKey+":"+startTime] = struct{}{}
	} else if !hasStartTimeKeyedObservation(s.seen, pidKey) {
		s.seen[pidKey] = struct{}{}
	}
	if envOK {
		s.byPID[pid] = env
	}
}

func hasStartTimeKeyedObservation(seen map[string]struct{}, pidKey string) bool {
	prefix := pidKey + ":"
	for key := range seen {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
