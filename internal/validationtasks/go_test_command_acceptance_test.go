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
		Entry("when a comment follows", "go test -timeout 25m ./... # -timeout 5m"),
		Entry("when a second command follows", "go test ./...; go test -timeout 25m ./..."),
	)
})
