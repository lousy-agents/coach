package projectcheck

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// recordingReporter stands in for *testing.T so that a test can see what a fake would have reported: the
// failures it recorded, and the cleanups it registered, which run when the test ends.
type recordingReporter struct {
	failures []string
	cleanups []func()
}

func (r *recordingReporter) Helper()          {}
func (r *recordingReporter) Cleanup(f func()) { r.cleanups = append(r.cleanups, f) }
func (r *recordingReporter) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *recordingReporter) finish() {
	for _, cleanup := range r.cleanups {
		cleanup()
	}
}

// The fake that fails one question does not answer the others with empty output: a question production
// starts asking is an error that names it.
func TestTheFakeThatFailsOneQuestionFailsAQuestionNobodyListed(t *testing.T) {
	run := questionsWhereOnly(&recordingReporter{}, theRevisionCheck)

	_, err := run("repo", "frobnicate", "--now")

	if err == nil || !strings.Contains(err.Error(), "frobnicate --now") {
		t.Errorf("run = %v; want an error that names the unexpected question", err)
	}
}

// A test whose code stops before the question that was to fail has not tested that question. The fake holds
// the test to asking it.
func TestTheFakeThatFailsOneQuestionFailsTheTestThatNeverAskedIt(t *testing.T) {
	reporter := &recordingReporter{}
	run := questionsWhereOnly(reporter, theRevisionCheck)
	_, _ = run("repo", "ls-tree", "-z", "--full-tree", "HEAD", "--", ":(top,literal)package.json")

	reporter.finish()

	if len(reporter.failures) != 1 || !strings.Contains(reporter.failures[0], "never asked") {
		t.Errorf("failures = %q; want the one that says the question to fail was never asked", reporter.failures)
	}
}

func TestTheFakeThatFailsOneQuestionLeavesTheTestThatAskedItAlone(t *testing.T) {
	reporter := &recordingReporter{}
	run := questionsWhereOnly(reporter, theRevisionCheck)
	_, err := run("repo", "cat-file", "-e", "HEAD^{commit}")

	reporter.finish()

	if !errors.Is(err, errNoAnswer) || len(reporter.failures) != 0 {
		t.Errorf("err = %v, failures = %q; want the named question to fail and nothing reported", err, reporter.failures)
	}
}

const (
	listedQuestion      = "cat-file -e HEAD^{commit}"
	listedQuestionKind  = "cat-file -e HEAD~1^{commit}"
	failingListedAnswer = "ls-tree -z --full-tree HEAD -- :(top,literal)cfg/p.json"
)

// A listed question is matched by its whole argument list, so a question of the same kind about something
// else is one nobody listed.
func TestTheFakeWithAListOfAnswersFailsAQuestionOfTheSameKindThatNobodyListed(t *testing.T) {
	run := questionsAnsweredExactly(&recordingReporter{}, map[string]gitAnswer{listedQuestion: {}})

	_, err := run("repo", "cat-file", "-e", "HEAD~1^{commit}")

	if err == nil || !strings.Contains(err.Error(), listedQuestionKind) {
		t.Errorf("run = %v; want an error that names the question that was not listed", err)
	}
}

func TestTheFakeWithAListOfAnswersGivesTheListedAnswerToTheListedQuestion(t *testing.T) {
	reporter := &recordingReporter{}
	run := questionsAnsweredExactly(reporter, map[string]gitAnswer{
		listedQuestion:      {output: "listed output"},
		failingListedAnswer: {err: errNoAnswer},
	})

	output, outputErr := run("repo", "cat-file", "-e", "HEAD^{commit}")
	_, failure := run("repo", "ls-tree", "-z", "--full-tree", "HEAD", "--", ":(top,literal)cfg/p.json")
	reporter.finish()

	if string(output) != "listed output" || outputErr != nil || !errors.Is(failure, errNoAnswer) || len(reporter.failures) != 0 {
		t.Errorf("output = %q, %v, failure = %v, reported = %q; want each listed answer and nothing reported", output, outputErr, failure, reporter.failures)
	}
}

// The list is the path the code is to take: a listed question that is never asked means it took another.
func TestTheFakeWithAListOfAnswersFailsTheTestThatNeverAskedAListedQuestion(t *testing.T) {
	reporter := &recordingReporter{}
	run := questionsAnsweredExactly(reporter, map[string]gitAnswer{listedQuestion: {}, failingListedAnswer: {err: errNoAnswer}})
	_, _ = run("repo", "cat-file", "-e", "HEAD^{commit}")

	reporter.finish()

	if len(reporter.failures) != 1 || !strings.Contains(reporter.failures[0], failingListedAnswer) {
		t.Errorf("failures = %q; want the one that names the listed question nobody asked", reporter.failures)
	}
}
