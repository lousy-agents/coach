package evaldetect_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

func channelFinding(path, pattern string, line int) evaldetect.Finding {
	return evaldetect.Finding{RuleID: evaldetect.GradingChannelRule, Path: path, Subject: evaldetect.GradingChannelRule + ":" + pattern, Line: line}
}

func channelScan(files map[string]string) ([]evaldetect.Finding, error) {
	return evaldetect.GradingChannelAccess(writeTree(GinkgoT().TempDir(), files))
}

var _ = Describe("the G2 static check for grading-channel access (#857, design 4 and 7)", func() {
	DescribeTable("flags a Go reference and keeps its look-alike clear",
		func(source string, want []evaldetect.Finding) {
			findings, err := channelScan(map[string]string{"pkg/a/a.go": "package a\n\n" + source})

			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(Equal(want))
		},
		Entry("os.NewFile on an inherited descriptor", "import \"os\"\n\nvar f = os.NewFile(3, \"in\")\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "os_new_file", 5)}),
		Entry("a NewFile of another package", "import \"example.com/store\"\n\nvar f = store.NewFile(3, \"in\")\n", nil),
		Entry("another os call", "import \"os\"\n\nvar f = os.Getenv(\"HOME\")\n", nil),
		Entry("os.NewFile named only in a comment", "// os.NewFile(3) is how a test could reach it\nvar x = 1\n", nil),
		Entry("os.NewFile named only in a string", "var x = \"os.NewFile\"\n", nil),
		Entry("a /proc/self/fd path", "var p = \"/proc/self/fd/3\"\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "proc_fd", 3)}),
		Entry("a /proc/<pid>/fd path", "var p = \"/proc/1234/fd/4\"\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "proc_fd", 3)}),
		Entry("a /proc/<ppid>/fd format string", "var p = \"/proc/%d/fd/4\"\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "proc_fd", 3)}),
		Entry("the /proc/self/fdinfo look-alike", "var p = \"/proc/self/fdinfo/3\"\n", nil),
		Entry("a /proc/self/mem path", "var p = \"/proc/self/mem\"\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "proc_mem", 3)}),
		Entry("a /proc/<pid>/mem path", "var p = \"/proc/1/mem\"\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "proc_mem", 3)}),
		Entry("a /proc/ prefix being built up", "var p = \"/proc/\" + pid\n", []evaldetect.Finding{channelFinding("pkg/a/a.go", "proc_path_built", 3)}),
		Entry("a /proc path that only begins with mem", "var p = \"/proc/self/memlog\"\n", nil),
		Entry("the /proc/meminfo look-alike", "var p = \"/proc/meminfo\"\n", nil),
		Entry("the /proc/cpuinfo look-alike", "var p = \"/proc/cpuinfo\"\n", nil),
		Entry("the /proc/self/status look-alike", "var p = \"/proc/self/status\"\n", nil),
	)

	DescribeTable("flags a TypeScript reference and keeps its look-alike clear",
		func(source string, want []evaldetect.Finding) {
			findings, err := channelScan(map[string]string{"src/a/a.ts": source})

			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(Equal(want))
		},
		Entry("process.binding", "const b = process.binding(\"fs\");\n", []evaldetect.Finding{channelFinding("src/a/a.ts", "process_binding", 1)}),
		Entry("process.bindings, which is not it", "const b = process.bindings;\n", nil),
		Entry("another object's binding", "const b = myprocess.binding(\"fs\");\n", nil),
		Entry("process.binding in a line comment", "// process.binding(\"fs\")\nconst b = 1;\n", nil),
		Entry("process.binding in a block comment", "/* process.binding(\"fs\") */\nconst b = 1;\n", nil),
		Entry("process.binding in a string", "const b = \"process.binding\";\n", nil),
		Entry("a write to an inherited descriptor", "import fs from \"node:fs\";\nfs.writeSync(4, \"x\");\n", []evaldetect.Finding{channelFinding("src/a/a.ts", "inherited_fd", 2)}),
		Entry("a read of an inherited descriptor", "import fs from \"node:fs\";\nconst n = fs.readFileSync(3, \"utf8\");\n", []evaldetect.Finding{channelFinding("src/a/a.ts", "inherited_fd", 2)}),
		Entry("a close of an inherited descriptor", "import fs from \"node:fs\";\nfs.closeSync(5);\n", []evaldetect.Finding{channelFinding("src/a/a.ts", "inherited_fd", 2)}),
		Entry("a write to standard output", "import fs from \"node:fs\";\nfs.writeSync(1, \"x\");\n", nil),
		Entry("a write to standard error", "import fs from \"node:fs\";\nfs.writeSync(2, \"x\");\n", nil),
		Entry("a read of a path", "import fs from \"node:fs\";\nconst n = fs.readFileSync(\"data.txt\", \"utf8\");\n", nil),
		Entry("a read of a path that is a number", "import fs from \"node:fs\";\nconst n = fs.readFileSync(\"4\");\n", nil),
		Entry("process and binding as two arguments", "use(process, binding);\n", nil),
		Entry("a read of a variable descriptor", "import fs from \"node:fs\";\nconst n = fs.readSync(fd, buffer);\n", nil),
		Entry("a /proc/self/fd string", "const p = \"/proc/self/fd/3\";\n", []evaldetect.Finding{channelFinding("src/a/a.ts", "proc_fd", 1)}),
		Entry("a /proc/<ppid>/fd template, whose /proc/ head is a path being built", "const p = `/proc/${process.ppid}/fd/4`;\n", []evaldetect.Finding{channelFinding("src/a/a.ts", "proc_path_built", 1)}),
		Entry("a /proc/<pid>/mem string", "const p = \"/proc/1/mem\";\n", []evaldetect.Finding{channelFinding("src/a/a.ts", "proc_mem", 1)}),
		Entry("a /proc/ prefix being built up", "const p = \"/proc/\" + process.ppid + \"/fd/4\";\n", []evaldetect.Finding{channelFinding("src/a/a.ts", "proc_path_built", 1)}),
		Entry("a bare writeSync to an inherited descriptor", "import { writeSync } from \"node:fs\";\nwriteSync(4, \"x\");\n", []evaldetect.Finding{channelFinding("src/a/a.ts", "inherited_fd", 2)}),
		Entry("a user function that is not an fs call", "function write(fd: number) { return fd; }\nwrite(4);\n", nil),
		Entry("a /proc/cpuinfo string", "const p = \"/proc/cpuinfo\";\n", nil),
		Entry("a /proc/meminfo template", "const p = `/proc/meminfo`;\n", nil),
	)

	It("reports each hit with its file and line, in path then line order", func() {
		findings, err := channelScan(map[string]string{
			"src/b/b.ts": "const x = 1;\nconst p = process.binding(\"fs\");\n",
			"src/a/a.ts": "\n\nconst p = \"/proc/self/mem\";\n",
			"pkg/z/z.go": "package z\n\nimport \"os\"\n\nvar f = os.NewFile(3, \"in\")\n",
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(findings).To(Equal([]evaldetect.Finding{
			channelFinding("pkg/z/z.go", "os_new_file", 5),
			channelFinding("src/a/a.ts", "proc_mem", 3),
			channelFinding("src/b/b.ts", "process_binding", 2),
		}))
	})

	DescribeTable("does not read test code, dependencies or hidden directories",
		func(path string) {
			findings, err := channelScan(map[string]string{path: "const b = process.binding(\"fs\");\nfs.writeSync(4, \"x\");\nvar f = os.NewFile(3, \"in\")\n"})

			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(BeEmpty())
		},
		Entry("a Go test", "pkg/a/a_test.go"),
		Entry("a TypeScript test", "src/a/a.test.ts"),
		Entry("a TypeScript spec", "src/a/a.spec.ts"),
		Entry("node_modules", "node_modules/x/x.ts"),
		Entry("a nested node_modules", "src/a/node_modules/x/x.ts"),
		Entry("git's own directory", ".git/hooks/x.ts"),
	)

	DescribeTable("reads a directory the agent can import from by relative path",
		func(path string) {
			findings, err := channelScan(map[string]string{path: "const b = process.binding(\"fs\");\n"})

			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(Equal([]evaldetect.Finding{channelFinding(path, "process_binding", 1)}))
		},
		Entry("a hidden directory", ".hidden/x.ts"),
		Entry("a nested hidden directory", "src/.hidden/x.ts"),
		Entry("a vendor directory", "vendor/x/x.ts"),
		Entry("a testdata directory", "src/testdata/x.ts"),
	)

	It("flags a reference in a .mts and a .cts source as in a .ts", func() {
		findings, err := channelScan(map[string]string{
			"src/a/a.mts": "const p = process.binding(\"fs\");\n",
			"src/a/b.cts": "const p = process.binding(\"fs\");\n",
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(findings).To(HaveLen(2))
	})

	It("refuses a .tsx source, whose JSX the lexer does not read", func() {
		_, err := channelScan(map[string]string{"src/a/a.tsx": "const a = <div />;\n"})

		Expect(errors.Is(err, evaldetect.ErrRefused)).To(BeTrue(), "got %v", err)
	})

	It("refuses a Go file it cannot parse, rather than skipping what it cannot read", func() {
		_, err := channelScan(map[string]string{"pkg/a/a.go": "package a\n\nfunc broken( {\n"})

		Expect(errors.Is(err, evaldetect.ErrRefused)).To(BeTrue(), "got %v", err)
	})

	It("refuses a TypeScript file whose slash it cannot classify, as the layer graph does", func() {
		_, err := channelScan(map[string]string{"src/a/a.ts": "const p = (1, /proc\\/self/.test(x));\n"})

		Expect(errors.Is(err, evaldetect.ErrRefused)).To(BeTrue(), "got %v", err)
	})

	It("reports nothing for a tree that holds no source", func() {
		findings, err := channelScan(map[string]string{"README.md": "process.binding os.NewFile /proc/self/fd/3"})

		Expect(err).NotTo(HaveOccurred())
		Expect(findings).To(BeEmpty())
	})
})
