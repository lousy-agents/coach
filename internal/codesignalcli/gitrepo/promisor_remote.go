package gitrepo

import (
	"strings"
)

// promisorRemoteSettings matches remote.<name>.promisor, the setting current git writes for a remote it
// fetches missing objects from.
const promisorRemoteSettings = `^remote\..*\.promisor$`

// promisorRemote is the remote a partial clone fetches missing objects from, and whether the repository
// is a partial clone at all. Git marks one in either of two ways: remote.<name>.promisor, which current
// git writes, and extensions.partialclone, which older git wrote to name the promisor remote.
//
// A setting git cannot read, or a configuration it cannot be asked, is an error and not a "no": a
// repository that cannot be told apart from an ordinary one must not be read as one.
func promisorRemote(run Runner, dir string) (remote string, found bool, err error) {
	remote, found, err = promisorByRemoteSetting(run, dir)
	if err != nil || found {
		return remote, found, err
	}
	return promisorByExtension(run, dir)
}

// promisorByRemoteSetting asks git to read each promisor setting as a boolean, so Coach agrees with git on
// what is true: a key with no value at all is true to git, and so is any non-zero number, and neither is
// the text "true".
func promisorByRemoteSetting(run Runner, dir string) (string, bool, error) {
	output, err := run(dir, "config", "--type=bool", "--get-regexp", promisorRemoteSettings)
	if err != nil {
		return "", false, noMatchIsNo(err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		key, value, split := cutAtLastSpace(strings.TrimSpace(line))
		if split && value == "true" {
			return strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".promisor"), true, nil
		}
	}
	return "", false, nil
}

func promisorByExtension(run Runner, dir string) (string, bool, error) {
	output, err := run(dir, "config", "--get", "extensions.partialclone")
	if err != nil {
		return "", false, noMatchIsNo(err)
	}
	remote := strings.TrimSpace(string(output))
	return remote, remote != "", nil
}

// noMatchIsNo is nil for git config's exit 1, which is how it says that nothing matched, and the error
// itself for any other failure (a damaged configuration, a time or output bound).
func noMatchIsNo(err error) error {
	if ConfigHasNoValue(err) {
		return nil
	}
	return err
}

// cutAtLastSpace splits a `key value` line at its last space, because a remote's name may hold spaces
// while a boolean never does.
func cutAtLastSpace(line string) (key, value string, split bool) {
	at := strings.LastIndex(line, " ")
	if at < 0 {
		return line, "", false
	}
	return line[:at], line[at+1:], true
}

// IsPartialClone reports whether the repository at dir was cloned with a filter, which is what makes git
// fetch a missing object on demand.
func IsPartialClone(run Runner, dir string) (bool, error) {
	_, found, err := promisorRemote(run, dir)
	return found, err
}

// PromisorRemote is the remote a partial clone fetches missing objects from, and false when the
// repository is not a partial clone or cannot be told to be one.
func PromisorRemote(run Runner, dir string) (string, bool) {
	remote, found, err := promisorRemote(run, dir)
	return remote, found && err == nil
}
