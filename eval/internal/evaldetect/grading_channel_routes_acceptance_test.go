package evaldetect_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

// fsDescriptorFunctions are the fs functions whose first argument is a descriptor; each is its own route to one.
var fsDescriptorFunctions = []string{"read", "write", "readFile", "writeFile", "appendFile", "close", "fstat", "ftruncate", "fsync", "fdatasync"}

var _ = Describe("every route the G2 check names to a descriptor, in both forms (review of #860)", func() {
	scanTS := func(path, source string) ([]evaldetect.Finding, error) {
		return channelScan(map[string]string{path: source})
	}

	for _, name := range fsDescriptorFunctions {
		It("flags fs."+name+" and fs."+name+"Sync on an inherited descriptor, and not on a path", func() {
			for _, call := range []string{name, name + "Sync"} {
				flagged, err := scanTS("src/a/a.ts", "import fs from \"node:fs\";\nfs."+call+"(4, x);\n")
				Expect(err).NotTo(HaveOccurred())
				Expect(flagged).To(Equal([]evaldetect.Finding{channelFinding("src/a/a.ts", "inherited_fd", 2)}), "fs."+call)

				clear, err := scanTS("src/a/a.ts", "import fs from \"node:fs\";\nfs."+call+"(\"data.txt\", x);\n")
				Expect(err).NotTo(HaveOccurred())
				Expect(clear).To(BeEmpty(), "fs."+call+" on a path")
			}
		})
	}

	DescribeTable("a descriptor written as another literal, or reached another way",
		func(source string, want []evaldetect.Finding) {
			findings, err := scanTS("src/a/a.ts", source)

			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(Equal(want))
		},
		Entry("a hexadecimal literal", "import fs from \"node:fs\";\nfs.writeSync(0x4, x);\n", []evaldetect.Finding{channelFinding("src/a/a.ts", "inherited_fd", 2)}),
		Entry("a hexadecimal literal below 3", "import fs from \"node:fs\";\nfs.writeSync(0x1, x);\n", nil),
		Entry("a /dev/fd path", "const p = \"/dev/fd/4\";\n", []evaldetect.Finding{channelFinding("src/a/a.ts", "proc_fd", 1)}),
		Entry("a /dev/fdx look-alike", "const p = \"/dev/fdx\";\n", nil),
		Entry("a /proc path with an extra segment before fd", "const p = \"/proc/a/b/fd\";\n", nil),
		Entry("a /proc/ that is only the end of a longer path", "const p = \"/x/proc/\";\n", nil),
	)

	DescribeTable("JavaScript that node loads is read like TypeScript",
		func(path string) {
			findings, err := scanTS(path, "const b = process.binding(\"fs\");\n")

			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(Equal([]evaldetect.Finding{channelFinding(path, "process_binding", 1)}))
		},
		Entry(".js", "src/a/a.js"),
		Entry(".mjs", "src/a/a.mjs"),
		Entry(".cjs", "src/a/a.cjs"),
	)

	DescribeTable("does not read a declaration file, which cannot run",
		func(path string) {
			findings, err := scanTS(path, "declare const b: typeof process.binding;\n")

			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(BeEmpty())
		},
		Entry(".d.ts", "src/a/a.d.ts"),
		Entry(".d.mts", "src/a/a.d.mts"),
		Entry(".d.cts", "src/a/a.d.cts"),
	)

	DescribeTable("a Go call into syscall or x/sys/unix on an inherited descriptor",
		func(source string, want []evaldetect.Finding) {
			findings, err := channelScan(map[string]string{"pkg/a/a.go": "package a\n\n" + source})

			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(Equal(want))
		},
		Entry("syscall.Write on descriptor 4", "import \"syscall\"\n\nvar n, err = syscall.Write(4, nil)\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "inherited_fd", 5)}),
		Entry("syscall.Dup on descriptor 4", "import \"syscall\"\n\nvar fd, err = syscall.Dup(4)\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "inherited_fd", 5)}),
		Entry("syscall.Read on a hexadecimal descriptor", "import \"syscall\"\n\nvar n, err = syscall.Read(0x4, nil)\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "inherited_fd", 5)}),
		Entry("unix.Write on descriptor 4", "import \"golang.org/x/sys/unix\"\n\nvar n, err = unix.Write(4, nil)\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "inherited_fd", 5)}),
		Entry("syscall imported under another name", "import sys \"syscall\"\n\nvar n, err = sys.Write(4, nil)\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "inherited_fd", 5)}),
		Entry("syscall.Write on descriptor 3, the first a test can inherit", "import \"syscall\"\n\nvar n, err = syscall.Write(3, nil)\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "inherited_fd", 5)}),
		Entry("syscall.Write on standard error", "import \"syscall\"\n\nvar n, err = syscall.Write(2, nil)\n", nil),
		Entry("syscall.Write on standard output", "import \"syscall\"\n\nvar n, err = syscall.Write(1, nil)\n", nil),
		Entry("syscall.Dup on standard error", "import \"syscall\"\n\nvar fd, err = syscall.Dup(2)\n", nil),
		Entry("a syscall call with no descriptor", "import \"syscall\"\n\nvar pid = syscall.Getpid()\n", nil),
		Entry("syscall.Write on a variable descriptor", "import \"syscall\"\n\nvar n, err = syscall.Write(fd, nil)\n", nil),
		Entry("another package named unix", "import \"example.com/unix\"\n\nvar n, err = unix.Write(4, nil)\n", nil),
		Entry("a cgo call on descriptor 4", "import \"C\"\n\nvar n = C.write(4, nil, 0)\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "inherited_fd", 5)}),
		Entry("a cgo call on standard output", "import \"C\"\n\nvar n = C.write(1, nil, 0)\n", nil),
		Entry("a call on a value named C when cgo is not imported", "var n = C.write(4, nil, 0)\n", nil),
		Entry("an aliased os import", "import o \"os\"\n\nvar f = o.NewFile(4, \"in\")\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "os_new_file", 5)}),
		Entry("a raw string that names /proc/<pid>/mem", "var p = `/proc/1/mem`\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "proc_mem", 3)}),
	)
})
