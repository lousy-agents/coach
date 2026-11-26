package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	coachBuild         sync.Once
	coachBinaryPath    string
	overrideBuild      sync.Once
	overrideBinaryPath string
)

// buildCoach builds the root module's coach CLI, with the given build tags,
// into the suite's build directory. The eval module never imports Coach; it
// runs this binary.
func buildCoach(name string, tags ...string) string {
	binary := filepath.Join(buildDir, name)
	args := []string{"build", "-o", binary}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	build := exec.Command("go", append(args, "./cmd/coach")...)
	build.Dir = filepath.Join("..", "..", "..")
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
	return binary
}

func coachBinary() string {
	coachBuild.Do(func() { coachBinaryPath = buildCoach("coach") })
	return coachBinaryPath
}

func coachOverrideBinary() string {
	overrideBuild.Do(func() { overrideBinaryPath = buildCoach("coach-override", "coach_eval_override") })
	return overrideBinaryPath
}

// isolatedGit runs git with no global or system configuration, so a fixture
// commit does not depend on the host's git setup.
func isolatedGit(dir string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-c", "user.name=fixture", "-c", "user.email=fixture@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
	return strings.TrimSpace(string(output))
}

// censusRepo commits files into a new repository and returns its path and
// the commit's SHA.
func censusRepo(files map[string]string) (string, string) {
	dir := GinkgoT().TempDir()
	isolatedGit(dir, "init", "-q")
	for path, content := range files {
		full := filepath.Join(dir, path)
		Expect(os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
		Expect(os.WriteFile(full, []byte(content), 0o644)).To(Succeed())
	}
	isolatedGit(dir, "add", "-A")
	isolatedGit(dir, "commit", "-q", "-m", "fixture")
	return dir, isolatedGit(dir, "rev-parse", "HEAD")
}

// complexFunctions returns a Go package with n functions whose cognitive
// complexity is 16, one above Coach's threshold of 15.
func complexFunctions(n int) string {
	var source strings.Builder
	source.WriteString("package p\n")
	for f := 0; f < n; f++ {
		source.WriteString("\nfunc F" + strconv.Itoa(f) + "(x int) int {\n\ty := 0\n")
		for i := 0; i < 16; i++ {
			source.WriteString("\tif x == " + strconv.Itoa(i) + " {\n\t\ty++\n\t}\n")
		}
		source.WriteString("\treturn y\n}\n")
	}
	return source.String()
}

func goRepoFiles(source string) map[string]string {
	return map[string]string{"go.mod": "module example.com/fx\n\ngo 1.26.0\n", "p/p.go": source}
}

type corpusRepo struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	SHA           string `json:"sha"`
	Language      string `json:"language"`
	ProjectConfig string `json:"project_config,omitempty"`
}

// writeCorpus writes a manifest that censuses cognitive complexity in Go.
func writeCorpus(repos ...corpusRepo) string {
	return writeCorpusFor("complexity.cognitive_complexity", "go", repos...)
}

// writeCorpusFor writes a manifest that censuses one rule in one language.
func writeCorpusFor(ruleID, language string, repos ...corpusRepo) string {
	manifest := map[string]any{
		"corpus_id":    "fixture",
		"repositories": repos,
		"rules":        []any{map[string]any{"rule_id": ruleID, "languages": []string{language}}},
	}
	raw, err := json.Marshal(manifest)
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(GinkgoT().TempDir(), "corpus.json")
	Expect(os.WriteFile(path, raw, 0o644)).To(Succeed())
	return path
}

type censusStatus struct {
	RuleID      string `json:"rule_id"`
	Status      string `json:"analysis_status"`
	Findings    int    `json:"findings"`
	Adjudicable int    `json:"adjudicable"`
	Evidence    string `json:"evidence"`
}

type censusSummary struct {
	CorpusID string `json:"corpus_id"`
	Coach    struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"coach"`
	Repositories []struct {
		ID       string         `json:"id"`
		SHA      string         `json:"sha"`
		Statuses []censusStatus `json:"statuses"`
	} `json:"repositories"`
}

func (s censusSummary) status(repoID string) censusStatus {
	for _, repo := range s.Repositories {
		if repo.ID == repoID {
			Expect(repo.Statuses).To(HaveLen(1))
			return repo.Statuses[0]
		}
	}
	Fail("no repository " + repoID + " in the census summary")
	return censusStatus{}
}

// runCensus runs coach-eval census over corpus with coach, adding env to
// the inherited environment.
func runCensus(corpus, coach string, env ...string) (compareOutcome, censusSummary) {
	outcome := runProgram("", append(os.Environ(), env...), coachEvalBinary, "census", "--coach", coach, corpus)
	var summary censusSummary
	if outcome.exitCode != 2 {
		Expect(json.Unmarshal([]byte(outcome.stdout), &summary)).To(Succeed(), "stdout: %s\nstderr: %s", outcome.stdout, outcome.stderr)
	}
	return outcome, summary
}
