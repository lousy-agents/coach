package gitrepo

import "os"

// SanitizedGitEnv returns the minimal environment for every git
// child that must stay local and hermetic, such as a snapshot read or a
// working-tree read. It never forwards the parent process's
// ambient environment wholesale: only PATH (so the git executable can be
// found) and HOME are carried through, plus GIT_TERMINAL_PROMPT/
// GIT_CONFIG_NOSYSTEM to force non-interactive, system-config-free
// behavior, and GIT_NO_LAZY_FETCH to disable partial-clone promisor fetches
// so a blobless snapshot can never reach the network. Go-tooling and
// proxy/credential variables (GOPROXY, GOFLAGS,
// GOPATH, GO111MODULE, GONOSUMCHECK, GOSUMDB, HTTP(S)_PROXY, NO_PROXY, ...)
// are deliberately never forwarded, even when set in the parent process:
// the reads that use this environment, snapshot reads (`git ls-tree`,
// `git show`) and working-tree reads (`git status`, `git ls-files`,
// `git config`), run against a local repository and need none of them, and
// forwarding them would be an unnecessary vector for those settings to
// influence a read that must stay local and hermetic.
func SanitizedGitEnv() []string {
	env := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_NO_LAZY_FETCH=1",
	}
	if value, ok := os.LookupEnv("PATH"); ok {
		env = append(env, "PATH="+value)
	}
	if value, ok := os.LookupEnv("HOME"); ok {
		env = append(env, "HOME="+value)
	}
	return env
}
