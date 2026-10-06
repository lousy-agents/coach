package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	linuxConfinementElsewhereSkip = "linux file-syscall and network-namespace controls run only on linux; they Fail, not Skip, in the ts-project-backend CI job"
	fileSyscallTracerUnavailable  = "file-syscall tracer unavailable; file-syscall control must Fail, not Skip, inside Label(\"ts-project-backend\")"
	unshareUnavailable            = "unshare -rn and privileged unshare -n both unavailable; network-namespace control must Fail, not Skip, inside Label(\"ts-project-backend\")"
	unconfinedArgvHook            = "--coach-test-unconfined"
	compilerModuleArgPrefix       = "--compiler-module="
	nativePackageArgPrefix        = "--native-package="
)

func ensureFileSyscallTracer() {
	if _, err := exec.LookPath("strace"); err == nil {
		return
	}
	_ = exec.Command("sudo", "apt-get", "update").Run()
	if err := exec.Command("sudo", "apt-get", "install", "-y", "strace").Run(); err != nil {
		Fail(fileSyscallTracerUnavailable)
	}
	if _, err := exec.LookPath("strace"); err != nil {
		Fail(fileSyscallTracerUnavailable)
	}
}

func runCoachBaselineUnderStrace(repo, coachPath, traceFile string) (stdout, stderr []byte, exitCode int) {
	args := []string{
		"-f",
		"-s", "65535",
		"-y",
		"-e", "trace=" + linuxStraceTraceExpr(),
		"-o", traceFile,
		"--",
		coachPath,
		"codesignal", "--baseline",
		"--project-config", "project.json",
		"--project-language", "typescript",
		"--format=json",
	}
	cmd := exec.Command("strace", args...)
	cmd.Dir = repo
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = []byte(outBuf.String())
	stderr = []byte(errBuf.String())
	if err == nil {
		return stdout, stderr, 0
	}
	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return stdout, stderr, exitErr.ExitCode()
}

func newLinuxTypeRootsFixture() (repo, decoy string) {
	repo = newTempGitRepo()
	decoyDir, err := os.MkdirTemp("", "coach-d2-typeroots-decoy-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, decoyDir)
	Expect(os.MkdirAll(filepath.Join(decoyDir, "child"), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(decoyDir, "child", "index.d.ts"), []byte("export {};\n"), 0o644)).To(Succeed())
	decoy = decoyDir

	version := realTypescriptVersion()
	tsconfig := fmt.Sprintf(`{"compilerOptions":{"module":"commonjs","moduleResolution":"node10","typeRoots":[%q],"types":["child"]}}`, filepath.ToSlash(decoy))
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
	commitFile(repo, "tsconfig.json", tsconfig+"\n")
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	installRealTypescriptCompiler(repo, true)
	return repo, decoy
}

func probedNodeExecPath() string {
	out, err := exec.Command("node", "-p", "process.execPath").Output()
	Expect(err).NotTo(HaveOccurred())
	path := strings.TrimSpace(string(out))
	Expect(filepath.IsAbs(path)).To(BeTrue(), "probed ExecPath must be absolute, got %q", path)
	return path
}

func repositoryRoot() string {
	_, file, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
