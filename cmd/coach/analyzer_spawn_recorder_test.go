package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
)

const analyzerArg = analyzerArgMarker + "/compiler"

// newTestRecorder installs a recorder whose "real node" prints "target:" and
// its arguments, so a test can tell the recorder passed a call through.
func newTestRecorder(t *testing.T) analyzerSpawnRecorder {
	t.Helper()
	g := NewWithT(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "echo-target")
	g.Expect(os.WriteFile(target, []byte("#!/bin/sh\necho \"target:$*\"\n"), 0o755)).To(Succeed())
	recorder, err := newAnalyzerSpawnRecorder(dir, target)
	g.Expect(err).NotTo(HaveOccurred())
	return recorder
}

// spawnAnalyzer returns its failure instead of failing the test so goroutines
// can call it.
func spawnAnalyzer(recorder analyzerSpawnRecorder, env []string) error {
	command := exec.Command(recorder.nodePath, "analyzer.js", analyzerArg)
	command.Env = env
	out, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("spawning the analyzer: %w: %s", err, out)
	}
	return nil
}

func TestAnalyzerSpawnRecorderCountsEveryShortLivedSpawn(t *testing.T) {
	const spawns = 50
	g := NewWithT(t)
	recorder := newTestRecorder(t)

	for range spawns {
		g.Expect(spawnAnalyzer(recorder, nil)).To(Succeed())
	}

	g.Expect(recorder.mustTakeSpawns(g)).To(HaveLen(spawns),
		"a launch that lives only a few milliseconds must still be counted")
}

func TestAnalyzerSpawnRecorderCountsConcurrentSpawnsIntact(t *testing.T) {
	const spawns = 16
	g := NewWithT(t)
	recorder := newTestRecorder(t)

	failures := make([]error, spawns)
	var launched sync.WaitGroup
	for i := range spawns {
		launched.Add(1)
		go func() {
			defer launched.Done()
			failures[i] = spawnAnalyzer(recorder, []string{"PATH=/runtime", "MARK=whole"})
		}()
	}
	launched.Wait()

	g.Expect(errors.Join(failures...)).To(Succeed())
	g.Expect(recorder.mustTakeSpawns(g)).To(And(
		HaveLen(spawns),
		HaveEach(And(
			HaveField("Environ", ContainSubstring("MARK=whole")),
			HaveField("PID", Not(BeZero())),
		)),
	), "every record must be complete, not interleaved with another launch's")
}

func TestAnalyzerSpawnRecorderRecordsTheEnvironmentAsTheShellExportsIt(t *testing.T) {
	g := NewWithT(t)
	recorder := newTestRecorder(t)

	g.Expect(spawnAnalyzer(recorder, []string{"PATH=/only/runtime", "ANALYZER_MARK=present"})).To(Succeed())

	taken := recorder.mustTakeSpawns(g)
	g.Expect(taken).To(HaveLen(1))
	g.Expect(pathValueFromEnviron(taken[0].Environ)).To(Equal("/only/runtime"), "recorded environ %q", taken[0].Environ)
	g.Expect(taken[0].Environ).To(ContainSubstring("ANALYZER_MARK=present"),
		"the record must hold the variable the spawn was given")
}

func TestAnalyzerSpawnRecorderTakesOnlyNewSpawns(t *testing.T) {
	g := NewWithT(t)
	recorder := newTestRecorder(t)

	g.Expect(spawnAnalyzer(recorder, nil)).To(Succeed())
	g.Expect(recorder.mustTakeSpawns(g)).To(HaveLen(1), "first take")

	g.Expect(spawnAnalyzer(recorder, nil)).To(Succeed())
	g.Expect(spawnAnalyzer(recorder, nil)).To(Succeed())
	g.Expect(recorder.mustTakeSpawns(g)).To(HaveLen(2), "second take: the 2 launched since the first take")

	g.Expect(recorder.mustTakeSpawns(g)).To(BeEmpty(), "third take: nothing launched since the second take")
}

func TestAnalyzerSpawnRecorderAnswersTheLauncherProbesWithoutCountingThem(t *testing.T) {
	g := NewWithT(t)
	recorder := newTestRecorder(t)

	execPath, err := exec.Command(recorder.nodePath, "-p", "process.execPath").Output()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(strings.TrimSpace(string(execPath))).To(Equal(recorder.nodePath),
		"the launcher spawns whatever path the probe reports, so it must be the recorder itself")

	version, err := exec.Command(recorder.nodePath, "--version").Output()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(strings.TrimSpace(string(version))).To(Equal("target:--version"),
		"--version must be passed through to the real node")

	g.Expect(recorder.mustTakeSpawns(g)).To(BeEmpty(), "the two probes are not analyzer launches")
}

func TestAnalyzerSpawnRecorderRunsTheRealNodeWithTheAnalyzerArguments(t *testing.T) {
	g := NewWithT(t)
	recorder := newTestRecorder(t)

	out, err := exec.Command(recorder.nodePath, "analyzer.js", analyzerArg).Output()

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(strings.TrimSpace(string(out))).To(Equal("target:analyzer.js "+analyzerArg),
		"the real node must receive the unmodified arguments")
}

func TestAnalyzerSpawnRecorderFailsTheLaunchWhenItCannotRecordIt(t *testing.T) {
	g := NewWithT(t)
	recorder := newTestRecorder(t)
	g.Expect(os.RemoveAll(recorder.spawnDir)).To(Succeed())

	var stdout, stderr bytes.Buffer
	command := exec.Command(recorder.nodePath, "analyzer.js", analyzerArg)
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()

	var exitErr *exec.ExitError
	g.Expect(errors.As(err, &exitErr)).To(BeTrue(), "a launch that cannot be recorded must fail, got %v", err)
	g.Expect(exitErr.ExitCode()).To(Equal(97))
	g.Expect(stderr.String()).To(ContainSubstring("analyzer spawn recorder: could not record launch"))
	g.Expect(stdout.String()).NotTo(ContainSubstring("target:"), "the real node must not run for a launch that was not recorded")
}

func TestAnalyzerSpawnRecorderLeavesOnlyFinishedRecordsBehind(t *testing.T) {
	g := NewWithT(t)
	recorder := newTestRecorder(t)

	g.Expect(spawnAnalyzer(recorder, nil)).To(Succeed())

	left, err := os.ReadDir(recorder.spawnDir)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(left).To(HaveLen(1))
	g.Expect(left[0].Name()).To(HaveSuffix(".rec"), "a record is only complete once it has its final name")
}

func TestAnalyzerSpawnRecorderTakesOnlyFinishedRecordFiles(t *testing.T) {
	g := NewWithT(t)
	recorder := newTestRecorder(t)
	finished := filepath.Join(recorder.spawnDir, ".rec.AbC123.rec")
	beingWritten := filepath.Join(recorder.spawnDir, ".rec.XyZ789")
	g.Expect(os.WriteFile(finished, []byte("4242\nPATH=/x\n"), 0o644)).To(Succeed())
	g.Expect(os.WriteFile(beingWritten, []byte("4243\nPATH=/y\n"), 0o644)).To(Succeed())

	g.Expect(recorder.mustTakeSpawns(g)).To(ConsistOf(analyzerSpawn{PID: 4242, Environ: "PATH=/x\n"}))
	g.Expect(beingWritten).To(BeAnExistingFile(), "a temporary name belongs to a launch that has not finished recording")
}

func TestAnalyzerSpawnRecorderReportsARecordItCannotParseAndKeepsIt(t *testing.T) {
	g := NewWithT(t)
	recorder := newTestRecorder(t)
	good := filepath.Join(recorder.spawnDir, ".rec.good01.rec")
	malformed := filepath.Join(recorder.spawnDir, ".rec.bad002.rec")
	g.Expect(os.WriteFile(good, []byte("4242\nPATH=/x\n"), 0o644)).To(Succeed())
	g.Expect(os.WriteFile(malformed, nil, 0o644)).To(Succeed())

	taken, err := recorder.takeSpawns()

	g.Expect(err).To(MatchError(ContainSubstring(".rec.bad002.rec")))
	g.Expect(taken).To(ConsistOf(analyzerSpawn{PID: 4242, Environ: "PATH=/x\n"}), "the readable records are still returned")
	g.Expect(malformed).To(BeAnExistingFile(), "a record that cannot be parsed is kept for diagnosis, not deleted")
}

func TestAnalyzerSpawnRecorderReportsASpawnDirectoryItCannotRead(t *testing.T) {
	g := NewWithT(t)
	recorder := newTestRecorder(t)
	g.Expect(os.RemoveAll(recorder.spawnDir)).To(Succeed())

	_, err := recorder.takeSpawns()

	g.Expect(err).To(HaveOccurred(), "a recorder that cannot be read must not look like a launch count of zero")
}

func TestNewAnalyzerSpawnRecorderReportsAnUnusableDirectoryAsAnError(t *testing.T) {
	g := NewWithT(t)
	notADirectory := filepath.Join(t.TempDir(), "not-a-directory")
	g.Expect(os.WriteFile(notADirectory, nil, 0o644)).To(Succeed())

	_, err := newAnalyzerSpawnRecorder(filepath.Join(notADirectory, "recorder"), "/bin/true")

	g.Expect(err).To(HaveOccurred())
}
