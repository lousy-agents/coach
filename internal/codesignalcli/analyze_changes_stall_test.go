package codesignalcli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// A --base scan asks git two or three questions per changed file: its content at the head, at the merge
// base, and its changed lines. The git here is a fake that is the whole of git for the test: it answers the
// questions the scan asks of these files, records each, and fails the test on any other, so a stall can be
// placed at one question and the scan's calls counted.

const (
	headSHA      = "1111111111111111111111111111111111111111"
	mergeBaseSHA = "2222222222222222222222222222222222222222"
)

type fakeGit struct {
	t *testing.T
	// answers maps a question ("show <rev>:<path>", "diff <path>") to its answer; an error is the answer
	// when the question is in failures.
	failures map[string]error
	asked    []string
}

func (g *fakeGit) run(_ string, args ...string) ([]byte, error) {
	question := g.questionOf(args)
	g.asked = append(g.asked, question)
	if failure, failing := g.failures[question]; failing {
		return nil, failure
	}
	if args[0] == "diff" {
		return []byte("@@ -1 +1 @@\n-package a\n+package a // changed\n"), nil
	}
	return []byte("package a\n"), nil
}

func (g *fakeGit) questionOf(args []string) string {
	switch args[0] {
	case "show":
		return "show " + args[1]
	case "diff":
		return "diff " + args[len(args)-1]
	}
	g.t.Errorf("unexpected git question %v: the fake answers only show and diff", args)
	return strings.Join(args, " ")
}

func installFakeGit(t *testing.T, fake *fakeGit) {
	t.Helper()
	original := runGit
	t.Cleanup(func() { runGit = original })
	runGit = fake.run
}

func timeoutOf(t *testing.T, question string) error {
	t.Helper()
	read := boundedRead(t, func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "sleep", "30") }, 1<<10, 200*time.Millisecond)
	return fmt.Errorf("git %s: %w", question, read)
}

func modifiedFiles(paths ...string) []gitrepo.SelectedFile {
	files := make([]gitrepo.SelectedFile, len(paths))
	for i, path := range paths {
		files[i] = gitrepo.SelectedFile{Path: path, Status: "modified", Language: "go"}
	}
	return files
}

// analyze runs a --base scan of the files and says whether it produced a report, and its error.
func analyze(files []gitrepo.SelectedFile) (reported bool, err error) {
	report, err := AnalyzeChanges(context.Background(), ".", headSHA, mergeBaseSHA, files, nil, nil, "", nil, nil)
	return report != nil, err
}

type stalledQuestion struct {
	name     string
	question string
	// asked is how many questions the scan may have asked when it stopped
	asked int
}

func (c stalledQuestion) run(t *testing.T) {
	fake := &fakeGit{t: t, failures: map[string]error{c.question: timeoutOf(t, strings.Fields(c.question)[0])}}
	installFakeGit(t, fake)

	reported, err := analyze(modifiedFiles("a.go", "b.go", "c.go"))

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) || reported {
		t.Fatalf("AnalyzeChanges = %v, %v; want the operational error that ends the scan", reported, err)
	}
	if len(fake.asked) != c.asked {
		t.Errorf("the scan asked %d questions %v, want %d: it must stop at the first file git did not answer for", len(fake.asked), fake.asked, c.asked)
	}
}

func TestAnalyzeChangesEndsWithTheFirstQuestionGitDoesNotAnswer(t *testing.T) {
	cases := []stalledQuestion{
		{"the head content of a file", "show " + headSHA + ":a.go", 1},
		{"the base content of a file", "show " + mergeBaseSHA + ":a.go", 2},
		{"the changed lines of a file", "diff a.go", 3},
	}
	for _, c := range cases {
		t.Run(c.name, c.run)
	}
}

func TestAnalyzeChangesEndsWithTheBaseContentOfARemovedFileGitDoesNotAnswer(t *testing.T) {
	fake := &fakeGit{t: t, failures: map[string]error{"show " + mergeBaseSHA + ":a.go": timeoutOf(t, "show")}}
	installFakeGit(t, fake)
	removed := []gitrepo.SelectedFile{{Path: "a.go", Status: "removed", Language: "go"}, {Path: "b.go", Status: "removed", Language: "go"}}

	reported, err := analyze(removed)

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) || reported || len(fake.asked) != 1 {
		t.Errorf("AnalyzeChanges = %v, %v after %v; want the operational error after the first question", reported, err, fake.asked)
	}
}

func TestAnalyzeChangesGoesOnPastAFileGitPrintedTooMuchOf(t *testing.T) {
	tooMuch := boundedRead(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "head", "-c", "5000", "/dev/zero")
	}, 1<<10, 10*time.Second)
	fake := &fakeGit{t: t, failures: map[string]error{"show " + headSHA + ":a.go": fmt.Errorf("git show: %w", tooMuch)}}
	installFakeGit(t, fake)

	reported, err := analyze(modifiedFiles("a.go", "b.go"))

	if err != nil || !reported {
		t.Fatalf("AnalyzeChanges = %v, %v; want a report: a file git printed too much of is that file's problem", reported, err)
	}
	if !strings.Contains(strings.Join(fake.asked, "\n"), "show "+headSHA+":b.go") {
		t.Errorf("the scan asked %v; want it to go on to b.go", fake.asked)
	}
}

// The changed lines of one file are asked for that path only: a path that looks like an option or a revision
// must not be read as one, so the question ends its options with --.
func TestTheChangedLinesOfAFileAreAskedAfterTheEndOfTheOptions(t *testing.T) {
	var asked []string
	original := runGit
	t.Cleanup(func() { runGit = original })
	runGit = func(_ string, args ...string) ([]byte, error) {
		asked = args
		return []byte(""), nil
	}

	_, diagnostic, stalled := computeChangedRanges(".", mergeBaseSHA, "--output=x.go")

	n := len(asked)
	if diagnostic != nil || stalled != nil || n < 3 || asked[n-2] != "--" || asked[n-1] != "--output=x.go" {
		t.Errorf("git was asked %v, want the path after a lone --: %v, %v", asked, diagnostic, stalled)
	}
}
