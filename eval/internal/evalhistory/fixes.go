package evalhistory

import (
	"fmt"
	"regexp"
	"strings"
)

// FixPattern is the pinned, case-insensitive heuristic that marks a commit
// message as a fix. It is recorded with every run, because its precision is
// what a reader must judge (f.9).
var FixPattern = regexp.MustCompile(`(?i)\b(bug\s?fix(es|ed)?|fix(es|ed)?|bugs?|defects?|regressions?|hotfix(es)?)\b|\b(closes|resolves)\s+#\d+`)

// FixCommit is one commit whose message matched FixPattern.
type FixCommit struct {
	SHA     string
	Subject string
}

// FixScan is the fixes found in a window, and how many non-merge commits the
// window held, so the pattern's match rate can be reported.
type FixScan struct {
	Commits int
	Fixes   []FixCommit
}

// FindFixes lists the non-merge commits after since up to until whose
// message matches FixPattern, oldest first.
func FindFixes(repoDir, since, until string) (FixScan, error) {
	for _, rev := range []string{since, until} {
		if _, err := git(repoDir, "cat-file", "-e", rev+"^{commit}"); err != nil {
			return FixScan{}, fmt.Errorf("%s is not a commit in %s", rev, repoDir)
		}
	}
	log, err := git(repoDir, "log", "--no-merges", "--reverse", "--format=%H%x1f%s%x1f%B%x1e", since+".."+until)
	if err != nil {
		return FixScan{}, err
	}
	var scan FixScan
	for _, entry := range strings.Split(log, "\x1e") {
		fields := strings.SplitN(strings.TrimLeft(entry, "\n"), "\x1f", 3)
		if len(fields) != 3 {
			continue
		}
		scan.Commits++
		if FixPattern.MatchString(fields[2]) {
			scan.Fixes = append(scan.Fixes, FixCommit{SHA: fields[0], Subject: fields[1]})
		}
	}
	return scan, nil
}
