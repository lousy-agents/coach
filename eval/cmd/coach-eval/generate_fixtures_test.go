package main_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	. "github.com/onsi/gomega"
)

type generateOutcome struct {
	exitCode int
	stdout   string
	stderr   string
}

type generatedCell struct {
	Layers    int    `json:"layers"`
	Packages  int    `json:"packages"`
	Entry     string `json:"entry"`
	Depth     int    `json:"depth"`
	Field     string `json:"field"`
	Direction string `json:"direction"`
	Trap      string `json:"trap"`
}

type generatedProgram struct {
	ProgramID    string        `json:"program_id"`
	Path         string        `json:"path"`
	OracleDir    string        `json:"oracle_dir"`
	Cache        string        `json:"cache"`
	Seed         int64         `json:"seed"`
	Replacements int           `json:"replacements"`
	Cell         generatedCell `json:"cell"`
}

type programCell struct {
	layers    int
	packages  int
	entry     string
	depth     int
	field     string
	direction string
	trap      string
}

func (c programCell) flags() []string {
	return []string{
		"--layers", fmt.Sprint(c.layers), "--packages", fmt.Sprint(c.packages), "--entry", c.entry,
		"--depth", fmt.Sprint(c.depth), "--field", c.field, "--direction", c.direction, "--trap", c.trap,
	}
}

func runGenerate(cacheHome string, extraEnv []string, args ...string) generateOutcome {
	return runGenerateBinary(coachEvalBinary, cacheHome, extraEnv, args...)
}

func runGenerateBinary(binary, cacheHome string, extraEnv []string, args ...string) generateOutcome {
	cmd := exec.Command(binary, append([]string{"generate"}, args...)...)
	cmd.Env = append(fixtureEnvironment(cacheHome), extraEnv...)
	return runGenerateCommand(cmd)
}

var (
	hostGoCache     string
	hostGoCacheOnce sync.Once
)

// fixtureEnvironment gives a run its own fixture cache and keeps the host's Go build cache. Go derives its
// build cache from XDG_CACHE_HOME, so without GOCACHE every run would compile the standard library and the
// pinned gocognit again.
func fixtureEnvironment(cacheHome string) []string {
	hostGoCacheOnce.Do(func() {
		output, err := exec.Command("go", "env", "GOCACHE").Output()
		Expect(err).NotTo(HaveOccurred())
		hostGoCache = strings.TrimSpace(string(output))
	})
	return append(os.Environ(), "XDG_CACHE_HOME="+cacheHome, "GOCACHE="+hostGoCache)
}

func runGenerateCommand(cmd *exec.Cmd) generateOutcome {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		Expect(err).NotTo(HaveOccurred())
	}
	code := 0
	if exitErr != nil {
		code = exitErr.ExitCode()
	}
	return generateOutcome{exitCode: code, stdout: stdout.String(), stderr: stderr.String()}
}

func generateOne(cacheHome string, args ...string) generatedProgram {
	outcome := runGenerate(cacheHome, nil, args...)
	Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
	return decodeGenerated(outcome.stdout)
}

func decodeAllGenerated(stdout string) []generatedProgram {
	var programs []generatedProgram
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		var program generatedProgram
		Expect(json.Unmarshal([]byte(line), &program)).To(Succeed(), line)
		programs = append(programs, program)
	}
	return programs
}

func decodeGenerated(stdout string) generatedProgram {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	Expect(lines).To(HaveLen(1), stdout)
	var program generatedProgram
	Expect(json.Unmarshal([]byte(lines[0]), &program)).To(Succeed(), stdout)
	return program
}

func readTree(root string) map[string][]byte {
	tree := map[string][]byte{}
	Expect(filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		content, readErr := os.ReadFile(path)
		tree[filepath.ToSlash(rel)] = content
		return readErr
	})).To(Succeed())
	return tree
}

// treeID is the definition of program_id: sha256 over the sorted "path NUL sha256(content) LF" records.
func treeID(tree map[string][]byte) string {
	paths := make([]string, 0, len(tree))
	for path := range tree {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		sum := sha256.Sum256(tree[path])
		fmt.Fprintf(hash, "%s\x00%s\n", path, hex.EncodeToString(sum[:]))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func copyTree(tree map[string][]byte, dest string) {
	for path, content := range tree {
		target := filepath.Join(dest, filepath.FromSlash(path))
		Expect(os.MkdirAll(filepath.Dir(target), 0o755)).To(Succeed())
		Expect(os.WriteFile(target, content, 0o644)).To(Succeed())
	}
}

func goIn(dir string, args ...string) generateOutcome {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off")
	return runGenerateCommand(cmd)
}
