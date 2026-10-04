package main

import (
	"io"

	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func authoringOutputFiles() (stdout, stderr *os.File, readStdout, readStderr func() string) {
	var err error
	stdout, err = os.CreateTemp(GinkgoT().TempDir(), "authoring-stdout")
	Expect(err).NotTo(HaveOccurred())
	stderr, err = os.CreateTemp(GinkgoT().TempDir(), "authoring-stderr")
	Expect(err).NotTo(HaveOccurred())

	read := func(f *os.File) string {
		_, err := f.Seek(0, 0)
		Expect(err).NotTo(HaveOccurred())
		data, err := io.ReadAll(f)
		Expect(err).NotTo(HaveOccurred())
		return string(data)
	}
	return stdout, stderr, func() string { return read(stdout) }, func() string { return read(stderr) }
}
