package gitrepo

import (
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
	probes       map[string]probeAnswer
	probed       []string
}

func (r *fakeRepository) run(_ string, args ...string) ([]byte, error) {
	switch args[0] {
	case "version":
		return []byte(r.version), r.versionErr
	case "config":
		return r.answerConfig(args)
	case "merge-base":
		return r.answerMergeBase()
	case "rev-list":
		return r.answerProbe(args)
	}
	return nil, exitStatus128
}

func (r *fakeRepository) answerConfig(args []string) ([]byte, error) {
	if r.configErr != nil {
		return nil, r.configErr
	}
	answer := r.extension
	if strings.Contains(strings.Join(args, " "), "--get-regexp") {
		answer = r.promisors
	}
	if answer == "" {
		return nil, exitStatusOne
	}
	return []byte(answer), nil
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
