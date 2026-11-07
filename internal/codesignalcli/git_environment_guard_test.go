package codesignalcli

import (
	"strings"
	"testing"
)

// gitCommandFile is the one production file that starts a git process: it builds the environment every
// git child of the scan starts with.
const gitCommandFile = "internal/codesignalcli/gitrepo/git_command.go"

// gitFixtureDir holds the helper the tests build repositories with. It is not production code and runs git
// with the environment the test chooses.
const gitFixtureDir = "internal/codesignalcli/internal/gitfixture/"

// Every git child of the scan takes its environment from gitrepo's gitCommand: that is where
// GIT_NO_LAZY_FETCH, GIT_TERMINAL_PROMPT and GIT_LFS_SKIP_SMUDGE are set, and where the caller's own settings
// are left alone or taken away. A package that replaced the environment of such a child would read the same
// repository through different settings than the rest of the scan, and one report would then describe two
// versions of the same file.
func TestNoPackageBuildsTheEnvironmentOfAGitChild(t *testing.T) {
	for _, place := range envAssignmentsOfFilesThatUseGitrepo(moduleProductionFiles(t)) {
		t.Errorf("%s assigns an Env; a git child built by gitrepo keeps the environment gitCommand gave it, so add nothing here and put a setting every read needs in gitrepo/git_command.go", place)
	}
}

func TestOnlyGitCommandNamesGitAsTheProgramToStart(t *testing.T) {
	for _, place := range gitSpawnsOutsideGitCommand(moduleProductionFiles(t)) {
		t.Errorf("%s starts git itself; build the child with gitrepo.ReadCommandContext so it starts with the environment of every other read", place)
	}
}

func envAssignmentsOfFilesThatUseGitrepo(files []productionFile) []string {
	var places []string
	for _, file := range files {
		if file.importsGitrepo() {
			places = append(places, file.envAssignments()...)
		}
	}
	return places
}

func gitSpawnsOutsideGitCommand(files []productionFile) []string {
	var places []string
	for _, file := range files {
		if !mayStartGit(file) {
			places = append(places, file.gitSpawns()...)
		}
	}
	return places
}

func mayStartGit(file productionFile) bool {
	return file.name == gitCommandFile || strings.HasPrefix(file.name, gitFixtureDir)
}
