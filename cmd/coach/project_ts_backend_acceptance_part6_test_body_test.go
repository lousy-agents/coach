package main

import (
	"time"
)

func body_projectTsBackendAcceptancePart6Test_37(s *analyzerEnvironSampler) {
	defer close(s.done)
	s.capture()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			s.capture()
			return
		case <-ticker.C:
			s.capture()
		}
	}
}
