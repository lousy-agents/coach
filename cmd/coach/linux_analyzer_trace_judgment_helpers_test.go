package main

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

func decoyHitsOn(rec straceRecord, decoy string) []string {
	var hits []string
	for _, p := range rec.Paths {
		if pathHasPrefix(p, decoy) {
			hits = append(hits, rec.Raw)
		}
	}
	return hits
}

func scanAnalyzerSubtreeProbes(recs []straceRecord, subtree map[int]struct{}, allow linuxAllowlist, decoy, repo string, requireDecoy bool) (decoyHits, leaks []string) {
	for _, rec := range recs {
		if _, in := subtree[rec.PID]; !in {
			continue
		}
		decoyHits = append(decoyHits, decoyHitsOn(rec, decoy)...)
		if requireDecoy {
			continue
		}
		if _, mut := linuxMutationSyscalls[rec.Syscall]; mut {
			Fail(fmt.Sprintf("mutation syscall in confined analyzer subtree: %s", rec.Raw))
		}
		leaks = append(leaks, probeLeaksOn(allow, rec, repo)...)
	}
	return decoyHits, leaks
}
