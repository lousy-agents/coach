package gitrepo

import "strings"

// ambientGitSettings are the variables a caller's environment can set that change how git reads one repository
// in a way Coach's own questions cannot be asked around.
//
// The four pathspec modes change what a pathspec means. Coach builds `:(top,literal)<path>` to ask about one
// policy, and under GIT_LITERAL_PATHSPECS the pathspec matches nothing, so a policy that is committed reads as
// never committed, with its staged edit hidden and a new policy offered over it; GIT_ICASE_PATHSPECS made the
// same questions fail as unanswerable.
//
// GIT_CONFIG makes `git config`, and no other command, read one file. The check for a partial clone reads the
// repository's configuration with git config, so left set it answers from a different file than the reads it
// guards.
//
// What is left alone is everything that says where the repository, its work tree, its index and its objects
// are (GIT_DIR, GIT_WORK_TREE, GIT_COMMON_DIR, GIT_INDEX_FILE, GIT_OBJECT_DIRECTORY,
// GIT_ALTERNATE_OBJECT_DIRECTORIES, GIT_NAMESPACE, GIT_CEILING_DIRECTORIES, GIT_DISCOVERY_ACROSS_FILESYSTEM):
// they belong to whatever started Coach. A work tree whose git directory is elsewhere is found only through
// them, and so are the objects of a server-side hook, which arrive through a quarantine directory and the
// repository's own. v0.11.0 read them as the caller set them, and so does Coach. The same goes for what the
// caller needs for git to run and to find its configuration (PATH, HOME, XDG_CONFIG_HOME, GIT_CONFIG_GLOBAL,
// GIT_CONFIG_SYSTEM, GIT_CONFIG_COUNT with its keys and values, GIT_CONFIG_PARAMETERS, the locale and the
// temporary directory), which every git command reads alike, and for GIT_NO_REPLACE_OBJECTS and
// GIT_REPLACE_REF_BASE, the caller's choice of view of the history.
var ambientGitSettings = []string{
	"GIT_LITERAL_PATHSPECS", "GIT_GLOB_PATHSPECS", "GIT_NOGLOB_PATHSPECS", "GIT_ICASE_PATHSPECS",
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
