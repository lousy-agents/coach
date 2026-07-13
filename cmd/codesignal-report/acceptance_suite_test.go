package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCodeSignalReportAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "codesignal-report command acceptance suite")
}
