package main

import (
	. "github.com/onsi/ginkgo/v2"
)

func body_failOnIncompleteCoverageAcceptanceTest_83() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}
