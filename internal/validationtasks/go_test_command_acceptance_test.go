package validationtasks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("reading the go test timeout out of one command line (#909)", func() {
	DescribeTable("names the timeout go test itself receives",
		func(command string, minutes int) {
			got, named := goTestTimeoutMinutes(command)

			Expect(named).To(BeTrue())
			Expect(got).To(Equal(minutes))
		},
		Entry("as a separate argument", "go test -race -timeout 25m ./...", 25),
		Entry("as -timeout=value", "go test -race -timeout=30m ./...", 30),
		Entry("under go test's own spelling of the flag", "go test -race -test.timeout 30m ./...", 30),
		Entry("with two dashes", "go test -race --timeout 30m ./...", 30),
		Entry("when a package is named timeout, which is not a flag", "go test -timeout 25m ./... timeout", 25),
		Entry("when acceptance-ci-env forwards it", "mise exec -- go run ./cmd/acceptance-ci-env -timeout 25m ./...", 25),
	)

	DescribeTable("names no timeout",
		func(command string) {
			_, named := goTestTimeoutMinutes(command)

			Expect(named).To(BeFalse())
		},
		Entry("when the flag is absent", "go test -race ./..."),
		Entry("when the command is not go test", "echo -timeout 25m"),
		Entry("when the value is not in minutes", "go test -timeout 1h ./..."),
		Entry("when the test binary is given arguments after -args, which can override it", "go test -race -timeout 25m ./... -args -test.timeout 5m"),
		Entry("when the flag itself comes after -args", "go test ./... -args -timeout 25m"),
		Entry("when a later -timeout would win", "go test -timeout 25m ./... -timeout 5m"),
		Entry("when a later -test.timeout would win", "go test -timeout 25m ./... -test.timeout 5m"),
		Entry("when a later -test.timeout=value would win", "go test -timeout 25m ./... -test.timeout=5m"),
		Entry("when a -test.timeout comes before the package list and -timeout after it", "go test -race -test.timeout 5m -timeout 25m ./..."),
		Entry("when a bare -test.timeout comes after the package list", "go test -race -timeout 25m ./... -test.timeout 5m"),
		Entry("when a later two-dash --timeout=value would win", "go test -timeout 25m ./... --timeout=5m"),
		Entry("when a later two-dash --test.timeout would win", "go test -timeout 25m ./... --test.timeout 5m"),
		Entry("when a later 30m repeats the flag, because the parser names a timeout only when the flag is spelled once", "go test -timeout 25m ./... -timeout 30m"),
		Entry("when acceptance-ci-env forwards a later -test.timeout", "mise exec -- go run ./cmd/acceptance-ci-env -timeout 25m ./... -test.timeout 5m"),
		Entry("when the flag has no value", "go test ./... -timeout"),
		Entry("when a comment follows", "go test -timeout 25m ./... # -timeout 5m"),
		Entry("when a second command follows", "go test ./...; go test -timeout 25m ./..."),
	)
})
