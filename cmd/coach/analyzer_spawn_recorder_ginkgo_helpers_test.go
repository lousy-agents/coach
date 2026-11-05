package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// recordAnalyzerSpawns puts a recorder ahead of the real node on PATH for the
// rest of the spec, so launches from a coach subprocess and from in-process
// calls are both recorded.
func recordAnalyzerSpawns() analyzerSpawnRecorder {
	recorder := newSpecAnalyzerSpawnRecorder()
	GinkgoT().Setenv("PATH", recorderPath(recorder, os.Getenv("PATH")))
	return recorder
}

func newSpecAnalyzerSpawnRecorder() analyzerSpawnRecorder {
	dir, err := os.MkdirTemp("", "coach-analyzer-recorder-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)
	recorder, err := newAnalyzerSpawnRecorder(dir, realHostNodeExecPath())
	Expect(err).NotTo(HaveOccurred())
	return recorder
}

func recorderPath(recorder analyzerSpawnRecorder, rest string) string {
	return filepath.Dir(recorder.nodePath) + string(os.PathListSeparator) + rest
}

// realHostNodeExecPath resolves node the way the launcher does, through
// process.execPath, so a version-manager shim is never the recorder's target.
// The probe gets the launcher's scrubbed environment so NODE_OPTIONS or proxy
// variables a spec plants for the analyzer never run in this helper.
func realHostNodeExecPath() string {
	onPath, err := exec.LookPath("node")
	Expect(err).NotTo(HaveOccurred())
	probe := exec.Command(onPath, "-p", "process.execPath")
	probe.Env = stubToolchainEnv(os.Getenv("PATH"))
	probed, err := probe.Output()
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(probed))
}
