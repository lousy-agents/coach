package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

// maxCandidateSuffix is how many names beside a policy are tried for its candidate before no command is offered.
const maxCandidateSuffix = 99

// goReplaceRemediation is the line for a Go scan whose policy exists and cannot be used: authoring writes a candidate
// at a free path beside it, and the line names the move that replaces the policy. It is empty when the repository
// cannot be found or no free name is left, because a command that authoring refuses is worse than none.
func goReplaceRemediation(dir string, policy policyref.Ref) string {
	policyPath, named := policy.NamedPath()
	if !named {
		policyPath = projectconfig.CanonicalPath
	}
	root, err := gitrepo.RepositoryRootWith(gitrepo.RunBytes, dir)
	if err != nil {
		return ""
	}
	candidate, found := freeCandidateBeside(root, policyPath)
	if !found {
		return ""
	}
	return tssetup.ReplaceGoPolicyRemediation(policyPath, candidate)
}

// freeCandidateBeside is the first of <name>.candidate.json, <name>.candidate-2.json, and so on, in the directory
// of policyPath, that nothing is at in the working tree under root: authoring is create-only, so a name that is taken
// is refused. A path that cannot be checked is not free.
func freeCandidateBeside(root, policyPath string) (string, bool) {
	directory, file := path.Split(policyPath)
	stem := strings.TrimSuffix(file, ".json")
	for attempt := 1; attempt <= maxCandidateSuffix; attempt++ {
		name := stem + ".candidate.json"
		if attempt > 1 {
			name = fmt.Sprintf("%s.candidate-%d.json", stem, attempt)
		}
		candidate := directory + name
		_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(candidate)))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return candidate, true
		case err != nil:
			return "", false
		}
	}
	return "", false
}
