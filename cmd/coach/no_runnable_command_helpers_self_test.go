package main

import "testing"

// The helper is itself held to what it must catch and what it must leave alone. Each command below is one the advice could print
// to carry out a step on the user's files; the first four are the ones the advice used to print, and the rest are the ways
// the same commands are written that a list of fixed strings misses (options before the subcommand, other whitespace, a bare
// mv, cp or rm, and the commands that stage, commit or discard).
func TestTheHelperCatchesTheCommandsThatCarryOutAStep(t *testing.T) {
	commands := []string{
		"git restore -- 'a.json'", "git mv a A", "mv -f a b", "git checkout --merge -- a",
		"git add -- ':(literal)p.json'", "git commit -m x", "mv -- cfg/p.candidate.json cfg/p.json", "cp cfg/p.candidate.json cfg/p.json",
		"cp -f a b", "rm cfg/p.json", "git -c core.ignorecase=false mv a A", "git --no-pager checkout x",
		"git  mv a A", "git\tmv a A", "git \\\n  mv a A", "git\nmv a A", "git reset --hard", "git stash",
		"GIT MV a A", "/usr/bin/git mv a A", "git rm --cached p.json", "git checkout --ours -- p.json", "top=\"$(git rev-parse --show-toplevel)\"",
		"git --work-tree=. restore p.json", "\x1b[31mgit rm p.json\x1b[0m", "gi\x1b[1mt mv a A", "git \x1b[1mmv a A",
		"git --git-dir /x mv a A", "git --work-tree /x restore p.json", "git --git-dir=/x --work-tree /y add p.json",
		"git --namespace ns commit -m x", "git -C \"my dir\" commit -m x", "git -c \"core.x=y z\" add p.json", "git -C 'my dir' restore p.json", "git --no-pager --git-dir /x\n  checkout p.json",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) { requireTheCommandCaught(t, command) })
	}
}

func requireTheCommandCaught(t *testing.T, command string) {
	t.Helper()
	if runnableCommandIn(command) == "" {
		t.Errorf("runnableCommandIn(%q) = none; want the command caught", command)
	}
}

// The words the advice says in place of a command are not commands, whatever git and the names of the steps they hold.
func TestTheHelperLeavesTheWordsOfTheAdviceAlone(t *testing.T) {
	words := []string{
		`the policy is committed as "cfg/p.json", but Coach reads "cfg/P.json", so it is not read: make git record it as "cfg/P.json" (rename it in the repository, or commit a rename you have staged), then commit and run again. If another file already exists at "cfg/P.json", move it aside first; with core.ignorecase set, git can overwrite it (project_config_invalid)`,
		`bring back "cfg/p.json" with the conflict markers git makes from the two sides, resolve the conflict in it, mark it resolved, and commit`,
		`accept the deletion by recording "cfg/p.json" as deleted and committing`,
		`move the file that blocks the path, then keep the version of "cfg/p.json" that the side being merged into holds by bringing it back, mark it resolved, and commit`,
		`coach codesignal --baseline --suggest-project-config --output cfg/p.candidate.json -- writes a candidate to "cfg/p.candidate.json" beside the policy, because authoring refuses a path that exists; compare it with "cfg/p.json", replace the policy with it, commit, and rerun`,
		`coach codesignal: the committed policy cannot be used: fix it, or move it aside and commit the move, then run guided policy authoring to write a new one.`,
		`install the Go version the go.mod names, or put it first on PATH; Coach does not download it`,
		`the index also holds a staged edit to it, which clearing the mark leaves alone, and the next run then offers to keep it or to drop it`,
		`git --git-dir names the repository to look at and git --work-tree the tree to compare it with, and neither is a step to carry out`,
		`ask git --git-dir /x what it holds, and then commit what is staged by hand`,
	}
	for _, text := range words {
		t.Run(text[:40], func(t *testing.T) { requireTheWordsLeftAlone(t, text) })
	}
}

func requireTheWordsLeftAlone(t *testing.T, text string) {
	t.Helper()
	if found := runnableCommandIn(text); found != "" {
		t.Errorf("runnableCommandIn flags %q in words that name no command", found)
	}
}
