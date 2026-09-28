package main

import (
	. "github.com/onsi/ginkgo/v2"
)

func body_projectTsRootScopeMismatchAcceptanceTest_18() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}
