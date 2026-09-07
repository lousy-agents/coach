//go:build foreignclone

package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GoReleaser snapshot foreign-repository TypeScript scan", func() {
	When("a GoReleaser snapshot artifact scans a clone that contains no js/semantics tree", func() {
		BeforeEach(func() {
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
		})

		It("completes the fixture layer finding using that snapshot binary", func() {
			if _, err := exec.LookPath("goreleaser"); err != nil {
				Fail("GoReleaser snapshot artifact is missing; the clone command must Fail, not Skip")
			}
			root := repositoryRoot()
			cmd := exec.Command("goreleaser", "release", "--snapshot", "--clean", "--skip=publish", "--skip=sign")
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "goreleaser snapshot: %s", out)

			bin := snapshotCoachBinary(root)
			Expect(bin).NotTo(BeEmpty(), "GoReleaser snapshot artifact is missing; the clone command must Fail, not Skip")
			GinkgoWriter.Printf("foreign-clone snapshot binary: %s\n", bin)

			repo := newTempGitRepo()
			_, err = os.Stat(filepath.Join(repo, "js", "semantics"))
			Expect(os.IsNotExist(err)).To(BeTrue(), "clone still has js/semantics/")
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			installRealTypescriptCompiler(repo, true)
			_, err = os.Stat(filepath.Join(repo, "js", "semantics"))
			Expect(os.IsNotExist(err)).To(BeTrue(), "clone still has js/semantics/")

			stdout, stderr, exitCode := runCoachBinaryBaseline(bin, repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].RuleID).To(Equal("architecture.layer_violation"))
			Expect(report.ProjectChanges[0].MachineEvidence).To(HaveKeyWithValue("importer", "pkg/handlers/h.ts"))
			Expect(report.ProjectChanges[0].MachineEvidence).To(HaveKeyWithValue("importee", "pkg/db/d.ts"))
		})
	})
})

func runCoachBinaryBaseline(bin, repo string, extraArgs ...string) (stdout, stderr []byte, exitCode int) {
	args := append([]string{"codesignal", "--baseline"}, extraArgs...)
	command := exec.Command(bin, args...)
	command.Dir = repo
	var outBuf, errBuf strings.Builder
	command.Stdout = &outBuf
	command.Stderr = &errBuf
	err := command.Run()
	stdout = []byte(outBuf.String())
	stderr = []byte(errBuf.String())
	if err == nil {
		return stdout, stderr, 0
	}
	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return stdout, stderr, exitErr.ExitCode()
}

func snapshotCoachBinary(root string) string {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	dist := filepath.Join(root, "dist")
	direct := filepath.Join(dist, fmt.Sprintf("coach_%s_%s", osName, arch), "coach")
	if st, err := os.Stat(direct); err == nil && !st.IsDir() {
		return direct
	}
	matches, _ := filepath.Glob(filepath.Join(dist, fmt.Sprintf("coach_%s_%s*", osName, arch), "coach"))
	for _, m := range matches {
		if st, err := os.Stat(m); err == nil && !st.IsDir() {
			return m
		}
	}
	archives, _ := filepath.Glob(filepath.Join(dist, fmt.Sprintf("coach_%s_%s*.tar.gz", osName, arch)))
	if len(archives) == 0 {
		return ""
	}
	extracted := extractCoachFromTarGz(archives[0])
	return extracted
}

func extractCoachFromTarGz(archive string) string {
	dir, err := os.MkdirTemp("", "coach-snapshot-extract-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)
	f, err := os.Open(archive)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()
	gz, err := gzip.NewReader(f)
	Expect(err).NotTo(HaveOccurred())
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		Expect(err).NotTo(HaveOccurred())
		if filepath.Base(hdr.Name) != "coach" || hdr.FileInfo().IsDir() {
			continue
		}
		dest := filepath.Join(dir, "coach")
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		Expect(err).NotTo(HaveOccurred())
		_, err = io.Copy(out, tr)
		_ = out.Close()
		Expect(err).NotTo(HaveOccurred())
		return dest
	}
	return ""
}
