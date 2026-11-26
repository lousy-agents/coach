package validationtasks

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	evalTaskInvocation = regexp.MustCompile(`\beval-[a-z0-9][a-z0-9-]*`)
	evalModuleLeg      = regexp.MustCompile(`-C\s+["']?\.?/?eval\b|\bcd\s+["']?\.?/?eval\b|working-directory:\s*["']?\.?/?eval\b|\./eval/|\beval/go\.(?:mod|sum)\b`)
	scheduleTrigger    = regexp.MustCompile(`(?m)^\s*schedule:`)
)

// evalWorkflowViolations lists each line of a workflow that runs a mise eval-*
// task or a leg of the eval module, plus the schedule trigger when the
// workflow has one. The evaluation runs only as local mise tasks.
func evalWorkflowViolations(name, text string) []string {
	var violations []string
	for index, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if task := evalTaskInvocation.FindString(line); task != "" {
			violations = append(violations, fmt.Sprintf("%s:%d runs the mise task %s", name, index+1, task))
		}
		if leg := evalModuleLeg.FindString(line); leg != "" {
			violations = append(violations, fmt.Sprintf("%s:%d runs an eval-module leg (%s)", name, index+1, leg))
		}
	}
	if len(violations) > 0 && scheduleTrigger.MatchString(text) {
		violations = append(violations, name+": a schedule trigger runs the evaluation")
	}
	return violations
}

// githubAutomationFiles lists the YAML files under .github/workflows and
// .github/actions, the two places GitHub runs repository automation from.
func githubAutomationFiles(githubDir string) []string {
	var files []string
	for _, sub := range []string{"workflows", "actions"} {
		_ = filepath.WalkDir(filepath.Join(githubDir, sub), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			if ext := filepath.Ext(path); ext == ".yml" || ext == ".yaml" {
				files = append(files, path)
			}
			return nil
		})
	}
	return files
}
