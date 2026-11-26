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

	. "github.com/onsi/gomega"
)

type generateOutcome struct {
	exitCode int
	stdout   string
	stderr   string
}

type generatedProgram struct {
	ProgramID string `json:"program_id"`
	Path      string `json:"path"`
	OracleDir string `json:"oracle_dir"`
	Cache     string `json:"cache"`
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
	cmd := exec.Command(coachEvalBinary, append([]string{"generate"}, args...)...)
	cmd.Env = append(os.Environ(), "XDG_CACHE_HOME="+cacheHome)
	cmd.Env = append(cmd.Env, extraEnv...)
	return runCommand(cmd)
}

func runCommand(cmd *exec.Cmd) generateOutcome {
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
	return runCommand(cmd)
}
