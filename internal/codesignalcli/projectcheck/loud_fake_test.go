package projectcheck

import (
	"strings"
	"testing"
)

// The fake that fails one question does not answer the others with empty output: a question production
// starts asking is an error that names it.
func TestTheFakeThatFailsOneQuestionFailsAQuestionNobodyListed(t *testing.T) {
	run := questionsWhereOnly(theRevisionCheck)

	_, err := run("repo", "frobnicate", "--now")

	if err == nil || !strings.Contains(err.Error(), "frobnicate --now") {
		t.Errorf("run = %v; want an error that names the unexpected question", err)
	}
}
