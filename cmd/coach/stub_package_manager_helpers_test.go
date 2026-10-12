package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	stubPackageManagerCwdLog = "cwd.log"
	stubPackageManagerEnvLog = "env.log"
)

// writeStubPackageManagerScript writes an executable `kind` script into a
// fresh temp directory that prints version on `--version` and exits non-zero
// on anything else, and returns that directory. It never installs anything:
// no spec drives a real install through a stub.
func writeStubPackageManagerScript(kind, version string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubmanager-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo %s; exit 0; fi\nexit 1\n", version)
	Expect(os.WriteFile(filepath.Join(dir, kind), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathWithStubNodeAndPackageManager returns pathWithStubNode's PATH with a
// stub `kind` reporting managerVersion ahead of it, so a spec controls the
// version checks.package_manager classifies rather than inheriting the
// host's.
func pathWithStubNodeAndPackageManager(nodeVersion, kind, managerVersion string) string {
	return writeStubPackageManagerScript(kind, managerVersion) + string(os.PathListSeparator) + pathWithStubNode(nodeVersion)
}

// writeRecordingStubPackageManagerScript extends
// writeStubPackageManagerScript with a record of the working directory and
// the environment variable names each invocation actually saw, so a spec can
// assert how the probe confined the subprocess rather than only what it
// returned.
func writeRecordingStubPackageManagerScript(kind, version string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-recordingmanager-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := fmt.Sprintf("#!/bin/sh\necho \"$PWD\" >> %q\nenv | sed 's/=.*//' >> %q\n"+
		"if [ \"$1\" = \"--version\" ]; then echo %s; exit 0; fi\nexit 1\n",
		filepath.Join(dir, stubPackageManagerCwdLog), filepath.Join(dir, stubPackageManagerEnvLog), version)
	Expect(os.WriteFile(filepath.Join(dir, kind), []byte(script), 0o755)).To(Succeed())
	return dir
}

func readStubPackageManagerCwds(managerDir string) []string {
	return readStubPackageManagerLog(managerDir, stubPackageManagerCwdLog)
}

func readStubPackageManagerEnv(managerDir string) []string {
	return readStubPackageManagerLog(managerDir, stubPackageManagerEnvLog)
}

func readStubPackageManagerLog(managerDir, name string) []string {
	data, err := os.ReadFile(filepath.Join(managerDir, name))
	Expect(err).NotTo(HaveOccurred(), "expected the stub package manager at %s to have recorded %s", managerDir, name)
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}
