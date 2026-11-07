//go:build unix

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A directory that someone prepared (unpacked from a tar archive, say) can hold a named pipe where git
// expects a file under .git, and git then blocks opening it for as long as nobody writes to it. A git
// read that has no time limit makes coach wait for that forever, so every read on a scan's path has a
// limit and an operational error that names the git question that did not answer. The pipe replaces the
// file by a rename, so nothing is deleted.
//
// Two files are replaced, one per stage of a scan. HEAD is read by every git command, so the first question
// a scan asks (is this a partial clone) is the one that does not answer. The branch's ref is read only by
// the questions that resolve HEAD, so the partial-clone question answers and the scan's own reads are the
// ones that do not.
type pipeSite struct {
	name     string
	file     func(repo string) string
	question string
	// suggestExit is how --suggest-project-config ends: it reports a git read that did not answer as its own
	// structured diagnostic (project_config_suggestion_snapshot_unavailable) and exits 3, but only for a read
	// of its own; the guard that runs before every mode fails it with the generic operational exit 1.
	suggestExit int
	// terminalFirst is set where --prepare-compiler cannot be reached: with no terminal it exits 2 before
	// it asks git anything, so only a read that every mode makes first (the guard's) can be made to hang.
	terminalFirst bool
}

var pipeSites = []pipeSite{
	{
		name:     ".git/HEAD",
		file:     func(repo string) string { return filepath.Join(repo, ".git", "HEAD") },
		question: `partial clone \(git config: git execution timed out after 30s\)`,

		suggestExit: 1,
	},
	{
		name: "the branch's ref under .git/refs/heads",
		file: func(repo string) string {
			return filepath.Join(repo, ".git", "refs", "heads", strings.TrimSpace(gitIn(repo, "symbolic-ref", "--short", "HEAD")))
		},
		question: `git rev-parse: .*timed out after 30s`,

		suggestExit:   3,
		terminalFirst: true,
	},
}

// pipeMode is one way to run coach, and the exit code it ends with at a site.
type pipeMode struct {
	name     string
	wantExit int
	args     []string
}

func (site pipeSite) modes() []pipeMode {
	modes := []pipeMode{
		{site.name + ": --baseline", 1, []string{"codesignal", "--baseline"}},
		{site.name + ": --base", 1, []string{"codesignal", "--base", "HEAD"}},
		{site.name + ": --check-project", 1, []string{"codesignal", "--baseline", "--check-project", "--project-language", "typescript"}},
		{site.name + ": --suggest-project-config", site.suggestExit, []string{"codesignal", "--baseline", "--suggest-project-config", "--output", "suggested.json"}},
	}
	if site.terminalFirst {
		return modes
	}
	return append(modes, pipeMode{site.name + ": --prepare-compiler", 1, []string{"codesignal", "--baseline", "--prepare-compiler", "--no-interactive", "--project-language", "typescript"}})
}

var _ = Describe("coach codesignal: a named pipe where git expects a file under .git", func() {
	// ceiling is the wall time a metadata read (config, rev-parse) is held to, plus a margin.
	const ceiling = 30*time.Second + 25*time.Second

	pipedRepo := func(site pipeSite) string {
		repo := commitScannableGoRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
		commitFile(repo, "project.json", singleRootPolicyJSON)
		file := site.file(repo)
		Expect(os.Rename(file, file+".orig")).To(Succeed())
		Expect(syscall.Mkfifo(file, 0o644)).To(Succeed())
		DeferCleanup(func() { _ = os.Remove(file); _ = os.Rename(file+".orig", file) })
		return repo
	}

	// Every scan waits out the 30 seconds one git read is held to, and none of them waits on another, so they
	// run at once: the spec waits as long as one scan does and not as long as all of them together. Each scan
	// keeps the three things it was held to when it ran alone.
	It("ends every mode at every site within the limit of its reads, with an operational error that names the git question", func() {
		var scans []pipeScan
		for _, site := range pipeSites {
			for _, mode := range site.modes() {
				scans = append(scans, pipeScan{name: mode.name, repo: pipedRepo(site), env: stubToolchainEnv(pathWithStubNode("v24.9.9")), args: mode.args, wantExit: mode.wantExit, question: site.question})
			}
		}

		results := runAtOnce(scans, ceiling+20*time.Second)

		Expect(pipeProblems(scans, results, ceiling)).To(BeEmpty())
	})
})

// pipeScan is one scan of a repository whose git cannot answer a question, and what it must end with.
type pipeScan struct {
	name     string
	repo     string
	env      []string
	args     []string
	wantExit int
	// question is a regular expression that the error on stderr must match: the git question that did not answer.
	question string
}

type pipeScanResult struct {
	stderr   []byte
	exitCode int
	elapsed  time.Duration
}

// runAtOnce starts every scan, waits for all of them, and returns their results in the order of the scans.
func runAtOnce(scans []pipeScan, limit time.Duration) []pipeScanResult {
	results := make([]pipeScanResult, len(scans))
	var running sync.WaitGroup
	for i, scan := range scans {
		running.Add(1)
		go func() {
			defer running.Done()
			stderr, exitCode, elapsed := runCoachWithin(scan.repo, scan.env, limit, scan.args...)
			results[i] = pipeScanResult{stderr: stderr, exitCode: exitCode, elapsed: elapsed}
		}()
	}
	running.Wait()
	return results
}

// pipeProblems says, for every scan, what it did not do: end within the ceiling, with its exit code, and with an
// error that names the git question that did not answer.
func pipeProblems(scans []pipeScan, results []pipeScanResult, ceiling time.Duration) []string {
	var problems []string
	for i, scan := range scans {
		result := results[i]
		if result.elapsed >= ceiling {
			problems = append(problems, fmt.Sprintf("%s: the command waited on git for %s (ceiling %s); stderr: %s", scan.name, result.elapsed, ceiling, result.stderr))
		}
		if result.exitCode != scan.wantExit {
			problems = append(problems, fmt.Sprintf("%s: exit code %d, want %d; stderr: %s", scan.name, result.exitCode, scan.wantExit, result.stderr))
		}
		if !regexp.MustCompile(scan.question).Match(result.stderr) {
			problems = append(problems, fmt.Sprintf("%s: the error must name the git question that did not answer (%s); stderr: %s", scan.name, scan.question, result.stderr))
		}
	}
	return problems
}

// runCoachWithin runs coach in repo and gives up on it after limit. An exit code of -1 means it was still
// running then; the stderr and the wall time are returned either way.
func runCoachWithin(repo string, env []string, limit time.Duration, args ...string) (stderr []byte, exitCode int, elapsed time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	command := exec.CommandContext(ctx, commandPath, args...)
	command.Dir, command.Env = repo, env
	var errBuf bytes.Buffer
	command.Stderr = &errBuf
	begin := time.Now()
	err := command.Run()
	elapsed = time.Since(begin)
	var exitErr *exec.ExitError
	if err != nil && ctx.Err() == nil && errors.As(err, &exitErr) {
		return errBuf.Bytes(), exitErr.ExitCode(), elapsed
	}
	if ctx.Err() != nil {
		return errBuf.Bytes(), -1, elapsed
	}
	return errBuf.Bytes(), 0, elapsed
}
