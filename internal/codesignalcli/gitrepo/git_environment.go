package gitrepo

import "strings"

// ambientGitSettings are the variables a caller's environment can set that change what git reads, or which
// repository it reads. Coach asks about one repository, named by `-C dir`, with pathspecs it builds to be
// literal, and its answers are about the committed bytes and the index of that repository. A tool that
// exports one of these for its children (git itself does, to a hook) would have Coach's answers depend on a
// setting it never saw: with GIT_LITERAL_PATHSPECS the pathspecs Coach builds match nothing, and a policy that
// is committed reads as never committed, with its staged edit hidden and a new policy offered over it.
//
// What is left alone is what the caller needs for git to run and to find its own configuration: PATH, HOME,
// XDG_CONFIG_HOME, GIT_CONFIG_GLOBAL, GIT_CONFIG_SYSTEM, GIT_CONFIG_COUNT with its keys and values and
// GIT_CONFIG_PARAMETERS, the locale and the temporary directory. Configuration the caller supplies those ways
// is read by every git command alike, so a read cannot see a setting that the check for a partial clone did
// not, and a container that sets safe.directory through them needs it kept. GIT_CONFIG is the exception: it is
// the legacy variable that makes `git config`, and no other command, read one file. The check reads the
// repository's configuration with git config, so left set it answers from that file and not from the
// configuration the other reads use, and a partial clone reads as a complete one. Two
// that do change what git shows are also left alone, GIT_NO_REPLACE_OBJECTS and GIT_REPLACE_REF_BASE: they
// are the caller's choice of view of the history, and removing the first would turn replacements on against
// what the caller asked for.
var ambientGitSettings = []string{
	// How every pathspec is read.
	"GIT_LITERAL_PATHSPECS", "GIT_GLOB_PATHSPECS", "GIT_NOGLOB_PATHSPECS", "GIT_ICASE_PATHSPECS",
	// Which repository, work tree and index are read, and how git finds them.
	"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_NAMESPACE",
	"GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM",
	// Which objects are read.
	"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
	// The one file git config reads, and git config alone.
	"GIT_CONFIG",
}

// withoutAmbientGitSettings is env without those variables.
func withoutAmbientGitSettings(env []string) []string {
	kept := make([]string, 0, len(env))
	for _, entry := range env {
		if !isAmbientGitSetting(entry) {
			kept = append(kept, entry)
		}
	}
	return kept
}

func isAmbientGitSetting(entry string) bool {
	name, _, _ := strings.Cut(entry, "=")
	for _, setting := range ambientGitSettings {
		if name == setting {
			return true
		}
	}
	return false
}
