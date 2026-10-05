package thinproof_test

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("the shared thin-offline-proof fixture, served by fakegithub.Handler and read via pkg/githubingest's public API", func() {
	It("reads the fixture file byte-for-byte and metadata-exact, using installation credentials for the Contents API read (AC for issue #79's Task 0.3 thin proof)", func() {
		body_fixtureAcceptanceTest_readsTheFixtureFileByteForByteAndMetadataExactUs_19()
	})
})
