package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var commandPath string

func encoded(content string) string { return base64.StdEncoding.EncodeToString([]byte(content)) }

func runCommand(input string) (*codesignal.Report, int, string) {
	command := exec.Command(commandPath)
	command.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	Expect(err).NotTo(HaveOccurred(), "command stderr: %s", stderr.String())
	var report codesignal.Report
	Expect(json.Unmarshal(stdout.Bytes(), &report)).To(Succeed(), "command stdout should be one JSON report: %s", stdout.String())
	return &report, 0, stderr.String()
}

func commandDiagnostic(report *codesignal.Report, kind, path string) bool {
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Kind == kind && diagnostic.Path == path {
			return true
		}
	}
	return false
}

var _ = BeforeSuite(func() {
	directory, err := os.MkdirTemp("", "codesignal-report-acceptance-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, directory)
	commandPath = filepath.Join(directory, "codesignal-report")
	build := exec.Command("mise", "x", "--", "go", "build", "-o", commandPath, ".")
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building the command: %s", output)
})

var _ = Describe("codesignal-report command", func() {
	When("given a valid NDJSON batch", func() {
		It("emits one report for analyzed file changes", func() {
			input := strings.Join([]string{
				`{"repository":"example/repo","revision":"abc123","base":"main"}`,
				`{"path":"state.go","language":"go","head_content":"` + encoded("package state\nfunc Update(input *int) { *input = 1 }\n") + `","changed_ranges":[{"start_row":1,"end_row":1}]}`,
				``,
			}, "\n")
			report, status, _ := runCommand(input)
			Expect(status).To(Equal(0))
			Expect(report.Scope).To(Equal(codesignal.Scope{Repository: "example/repo", Revision: "abc123", Base: "main"}))
			Expect(report.Summary.FilesAnalyzed).To(Equal(1))
			Expect(report.Signals).To(HaveLen(1))
			Expect(report.Signals[0].Changed).To(BeTrue())
		})
	})

	When("individual requests are malformed or unanalysable", func() {
		It("reports diagnostics and continues the batch", func() {
			input := strings.Join([]string{
				`not JSON`,
				`{"path":"missing-language"}`,
				`{"path":"bad.go","language":"go","head_content":"not-base64!!"}`,
				`{"path":"broken.go","language":"go","head_content":"` + encoded("package broken\nfunc {\n") + `"}`,
				`{"path":"unknown.go","language":"not-a-language","head_content":"` + encoded("package unknown\n") + `"}`,
				`{"path":"good.go","language":"go","head_content":"` + encoded("package good\nfunc Update(input *int) { *input = 1 }\n") + `"}`,
				``,
			}, "\n")
			report, status, _ := runCommand(input)
			Expect(status).To(Equal(0))
			Expect(commandDiagnostic(report, "malformed_scope_header", "")).To(BeTrue())
			Expect(commandDiagnostic(report, "malformed_file_request", "")).To(BeTrue())
			Expect(commandDiagnostic(report, "invalid_content_encoding", "bad.go")).To(BeTrue())
			Expect(commandDiagnostic(report, "syntax_errors", "broken.go")).To(BeTrue())
			Expect(commandDiagnostic(report, "analysis_failed", "unknown.go")).To(BeTrue())
			Expect(report.Summary.FilesAnalyzed).To(Equal(4))
			Expect(report.Signals).To(HaveLen(1))
		})
	})

	When("all source arrives inline over stdin", func() {
		It("produces its report without any configured external service", func() {
			input := strings.Join([]string{
				`{"repository":"offline/repo","revision":"local"}`,
				`{"path":"inline.go","language":"go","head_content":"` + encoded("package inline\nfunc Update(input *int) { *input = 1 }\n") + `"}`,
				``,
			}, "\n")
			report, status, stderr := runCommand(input)
			Expect(status).To(Equal(0))
			Expect(stderr).To(BeEmpty())
			Expect(report.Scope.Repository).To(Equal("offline/repo"))
			Expect(report.Signals).To(HaveLen(1))
		})
	})
})
