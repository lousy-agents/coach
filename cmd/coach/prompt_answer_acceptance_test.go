package main

import (
	"io"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func stdinWithTyped(typed string) *os.File {
	GinkgoHelper()
	reader, writer, err := os.Pipe()
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(reader.Close)
	_, err = writer.WriteString(typed)
	Expect(err).NotTo(HaveOccurred())
	Expect(writer.Close()).To(Succeed())
	return reader
}

var _ = Describe("answering a yes or no prompt", func() {
	It("reads only its own line, so answers typed ahead for the next prompt are still unread", func() {
		stdin := stdinWithTyped("y\nfirst answer to the next prompt\n")

		Expect(answeredYes(stdin)).To(BeTrue())

		rest, err := io.ReadAll(stdin)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(rest)).To(Equal("first answer to the next prompt\n"))
	})

	DescribeTable("treats only y or yes as consent",
		func(typed string, want bool) {
			Expect(answeredYes(stdinWithTyped(typed))).To(Equal(want))
		},
		Entry("y", "y\n", true),
		Entry("YES in capitals", "YES\n", true),
		Entry("yes with surrounding spaces", "  yes  \n", true),
		Entry("y with no trailing newline", "y", true),
		Entry("an empty line, which is the default", "\n", false),
		Entry("n", "n\n", false),
		Entry("no", "no\n", false),
		Entry("anything else", "maybe\n", false),
		Entry("end of input with nothing typed", "", false),
	)
})
