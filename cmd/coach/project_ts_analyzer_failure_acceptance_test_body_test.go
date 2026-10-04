package main

import (
	. "github.com/onsi/ginkgo/v2"
)

func body_projectTsAnalyzerFailureAcceptanceTest_112() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}
