package projectconfig

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// replayedGit answers a question with what a real git said to it when it was first asked, and fails the test on a question
// it was never recorded for.
type replayedGit struct {
	t       *testing.T
	answers map[string]recordedAnswer
}

type recordedAnswer struct {
	output []byte
	err    error
}

func (g replayedGit) run(_ string, args ...string) ([]byte, error) {
	answer, known := g.answers[strings.Join(args, " ")]
	if !known {
		g.t.Errorf("unexpected git question %v: the replay answers only what the real git was asked", args)
	}
	return answer.output, answer.err
}

// recordingGit asks the real git and keeps each answer.
type recordingGit struct{ answers map[string]recordedAnswer }

func (g *recordingGit) run(dir string, args ...string) ([]byte, error) {
	output, err := NewGitRunner()(dir, args...)
	g.answers[strings.Join(args, " ")] = recordedAnswer{output: output, err: err}
	return output, err
}

// Every question a read of the committed policy asks goes to the runner the read was given, `show` of the document
// included. The runner here answers from a recording and the directory it is asked about does not exist, so a question that
// went to a git of its own would fail on that directory and the read with it.
func TestALoadAsksTheRunnerItIsGivenEveryQuestionIncludingTheDocument(t *testing.T) {
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", `{"schema_version":"1","roots":["."]}`)
	recording := &recordingGit{answers: map[string]recordedAnswer{}}
	want, err := loadThrough(recording.run, repo, "HEAD", "cfg/p.json")
	if err != nil {
		t.Fatalf("loadThrough over the real git: %v", err)
	}
	absent := filepath.Join(t.TempDir(), "no-such-repository")

	got, err := loadThrough(replayedGit{t: t, answers: recording.answers}.run, absent, "HEAD", "cfg/p.json")

	if err != nil || string(got) != string(want) {
		t.Errorf("loadThrough = %q, %v; want %q from the runner it was given, whatever directory it names", got, err, want)
	}
}

func TestALoadForReadinessAsksTheRunnerItIsGivenEveryQuestionIncludingTheDocument(t *testing.T) {
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", `{"schema_version":"1","roots":["."]}`)
	recording := &recordingGit{answers: map[string]recordedAnswer{}}
	want, err := loadForReadinessThrough(recording.run, repo, "HEAD", "cfg/p.json")
	if err != nil {
		t.Fatalf("loadForReadinessThrough over the real git: %v", err)
	}
	absent := filepath.Join(t.TempDir(), "no-such-repository")

	got, err := loadForReadinessThrough(replayedGit{t: t, answers: recording.answers}.run, absent, "HEAD", "cfg/p.json")

	if err != nil || len(got.Roots) != len(want.Roots) || got.Roots[0] != want.Roots[0] {
		t.Errorf("loadForReadinessThrough = %+v, %v; want %+v from the runner it was given, whatever directory it names", got, err, want)
	}
}
