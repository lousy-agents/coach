package projectconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Parse on an empty document", func() {
	DescribeTable("returns an error rather than a zero Config",
		func(document []byte) {
			_, err := Parse(document)

			Expect(err).To(HaveOccurred())
		},
		Entry("nil", nil),
		Entry("an empty slice", []byte{}),
	)
})
