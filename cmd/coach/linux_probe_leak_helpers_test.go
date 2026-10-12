package main

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

func probeLeaksOn(allow linuxAllowlist, rec straceRecord, repo string) []string {
	if _, probe := linuxProbeSyscalls[rec.Syscall]; !probe {
		return nil
	}
	if !straceSucceeded(rec) {
		return nil
	}
	var leaks []string
	for _, p := range rec.Paths {
		if linuxProbeAllowed(allow, rec.Syscall, p) {
			continue
		}
		failIfRepoOpenOrListing(rec, repo, p)
		leaks = append(leaks, fmt.Sprintf("%s (path %s)", rec.Raw, p))
	}
	return leaks
}

func failIfRepoOpenOrListing(rec straceRecord, repo, path string) {
	_, openOrListing := linuxOpenOrListingSyscalls[rec.Syscall]
	if repo != "" && openOrListing && pathHasPrefix(path, repo) {
		Fail(fmt.Sprintf("successful open or listing under the fixture repository root; do not add an allowlist entry: %s (path %s)", rec.Raw, path))
	}
}
