package gitrepo

import (
	"regexp"
	"slices"
	"strings"
)

// probeAnswer is what the fake repository says to a rev-list probe of one revision.
type probeAnswer struct {
	output string
	err    error
}

// fakeRepository answers the questions the lazy-fetch guard asks git, and records the rev-list probes it
// was asked. An empty promisors, extension or mergeBase is git's exit 1: nothing matched.
type fakeRepository struct {
	version      string
	versionErr   error
	promisors    string
	extension    string
	configErr    error
	mergeBase    string
	mergeBaseErr error
	// unbornHead is a repository whose HEAD names nothing: no commits yet, or no repository at all.
	unbornHead bool
	// revParseErr and revParseOutput are what rev-parse --verify says of any name, when they are set.
	revParseErr    error
	revParseOutput string
	probes         map[string]probeAnswer
	probed         []string
	// mergeBaseAsked is the arguments of each merge-base question, so a test can see how the base was passed.
	mergeBaseAsked [][]string
}

func (r *fakeRepository) run(_ string, args ...string) ([]byte, error) {
	switch args[0] {
	case "version":
		return []byte(r.version), r.versionErr
	case "config":
		return r.answerConfig(args)
	case "merge-base":
		r.mergeBaseAsked = append(r.mergeBaseAsked, args)
		return r.answerMergeBase()
	case "rev-list":
		return r.answerProbe(args)
	case "rev-parse":
		return r.answerResolution()
	}
	return nil, exitStatus128
}

func (r *fakeRepository) answerConfig(args []string) ([]byte, error) {
	if r.configErr != nil {
		return nil, r.configErr
	}
	answer := r.extension
	if slices.Contains(args, "--get-regexp") {
		answer = linesMatching(r.promisors, args[len(args)-1])
	}
	if answer == "" {
		return nil, exitStatusOne
	}
	return []byte(answer), nil
}

// linesMatching is the lines of config, each "key value", whose key matches pattern: what git config
// --get-regexp prints, so the pattern the code hands to git is the one that is evaluated. The value is a boolean
// and holds no space, while a remote's name in the key may.
func linesMatching(config, pattern string) string {
	keys := regexp.MustCompile(pattern)
	var matching strings.Builder
	for _, line := range strings.Split(config, "\n") {
		at := strings.LastIndex(line, " ")
		if at > 0 && keys.MatchString(line[:at]) {
			matching.WriteString(line + "\n")
		}
	}
	return matching.String()
}

func (r *fakeRepository) answerMergeBase() ([]byte, error) {
	if r.mergeBaseErr != nil {
		return nil, r.mergeBaseErr
	}
	return []byte(r.mergeBase + "\n"), nil
}

func (r *fakeRepository) answerProbe(args []string) ([]byte, error) {
	r.probed = append(r.probed, strings.Join(args, " "))
	answer := r.probes[args[len(args)-1]]
	return []byte(answer.output), answer.err
}

const (
	promisorOrigin = "remote.origin.promisor true\n"
	oldGit         = "git version 2.43.0"
	currentGit     = "git version 2.44.0"
)

func missingIDs(ids ...string) string {
	var output strings.Builder
	for _, id := range ids {
		output.WriteString("?" + id + "\n")
	}
	return output.String()
}

const (
	firstMissing  = "0123456789012345678901234567890123456789"
	secondMissing = "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
)

const (
	mergeBaseSHA  = "feedfacefeedfacefeedfacefeedfacefeedface"
	sha256Missing = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func (r *fakeRepository) answerResolution() ([]byte, error) {
	switch {
	case r.revParseErr != nil:
		return nil, r.revParseErr
	case r.unbornHead:
		return nil, exitStatus128
	case r.revParseOutput != "":
		return []byte(r.revParseOutput), nil
	}
	return []byte(firstMissing + "\n"), nil
}
