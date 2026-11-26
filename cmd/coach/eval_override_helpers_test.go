package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	evalOverrideEnvironmentVariable = "COACH_EVAL_OVERRIDE"
	evalOverrideBannerPrefix        = "coach: eval override active: "
	evalOverrideBuildTag            = "coach_eval_override"
)

func buildCoachWithTags(tags string) string {
	directory, err := os.MkdirTemp("", "coach-eval-override-build-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, directory)

	binary := filepath.Join(directory, "coach")
	arguments := []string{"build", "-ldflags", "-s -w", "-o", binary}
	if tags != "" {
		arguments = append(arguments, "-tags", tags)
	}
	arguments = append(arguments, ".")

	build := exec.Command("go", arguments...)
	build.Dir = filepath.Join(repositoryRoot(), "cmd", "coach")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building coach (tags %q): %s", tags, output)
	return binary
}

func environmentWithOverride(override string) []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, evalOverrideEnvironmentVariable+"=") {
			environment = append(environment, entry)
		}
	}
	if override == "" {
		return environment
	}
	return append(environment, evalOverrideEnvironmentVariable+"="+override)
}

func baselineScanWithOverride(binary, repo, override string) (stdout, stderr []byte, exitCode int) {
	return runCoachBinary(binary, repo, environmentWithOverride(override), "codesignal", "--baseline", "--format=json")
}

func reportedRuleIDs(stdout []byte) []string {
	var report struct {
		Signals []struct {
			RuleID string `json:"rule_id"`
		} `json:"signals"`
	}
	Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)

	seen := map[string]bool{}
	for _, signal := range report.Signals {
		seen[signal.RuleID] = true
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func repoWithFiles(files map[string]string) string {
	repo := newTempGitRepo()
	for name, contents := range files {
		commitFile(repo, name, contents)
	}
	return repo
}

func bannerLines(stderr []byte) []string {
	var banners []string
	for _, line := range strings.Split(string(stderr), "\n") {
		if strings.HasPrefix(line, evalOverrideBannerPrefix) {
			banners = append(banners, line)
		}
	}
	return banners
}
