package evalhistory_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// history is a fixture repository built one commit at a time.
type history struct {
	dir string
}

func newHistory() history {
	h := history{dir: GinkgoT().TempDir()}
	h.git("init", "-q", "-b", "main")
	return h
}

func (h history) git(args ...string) string {
	cmd := exec.Command("git", append([]string{"-c", "user.name=fixture", "-c", "user.email=fixture@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = h.dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
	return strings.TrimSpace(string(output))
}

// commit writes files (an empty content deletes the file) and commits them
// with message, returning the commit's SHA.
func (h history) commit(message string, files map[string]string) string {
	for path, content := range files {
		full := filepath.Join(h.dir, path)
		if content == "" {
			Expect(os.Remove(full)).To(Succeed())
			continue
		}
		Expect(os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
		Expect(os.WriteFile(full, []byte(content), 0o644)).To(Succeed())
	}
	h.git("add", "-A")
	h.git("commit", "-q", "--allow-empty", "-m", message)
	return h.git("rev-parse", "HEAD")
}

// baseSource has func A on lines 3-5 and func B on lines 7-10.
const baseSource = "package p\n\nfunc A() int {\n\treturn 1\n}\n\nfunc B() int {\n\tx := 2\n\treturn x\n}\n"

func lines(source string) []string {
	return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
}

func join(all []string) string { return strings.Join(all, "\n") + "\n" }

// replaceLine replaces 1-based line n.
func replaceLine(source string, n int, text string) string {
	all := lines(source)
	all[n-1] = text
	return join(all)
}

// insertLine inserts text after 1-based line n.
func insertLine(source string, n int, text string) string {
	all := lines(source)
	return join(append(all[:n:n], append([]string{text}, all[n:]...)...))
}

// deleteLine deletes 1-based line n.
func deleteLine(source string, n int) string {
	all := lines(source)
	return join(append(all[:n-1:n-1], all[n:]...))
}

// sumFunc and otherFunc are large enough for git's move and copy detection,
// which ignores blocks of fewer than 20 alphanumeric characters.
const sumFunc = "func Sum(values []int) int {\n\ttotal := 0\n\tfor _, value := range values {\n\t\ttotal += value\n\t}\n\treturn total\n}\n"

const otherFunc = "func Other(name string) string {\n\tgreeting := \"hello, \" + name\n\treturn greeting\n}\n"
