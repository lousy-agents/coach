package evalharness

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

// Exit codes of `coach-eval generate` (design §10).
const (
	GenerateOK           = 0
	GenerateFailed       = 1
	GenerateInputInvalid = 2
	GenerateRejectedMost = 8
)

// Generate emits one program per requested cell, from the cache when it is intact, and prints one JSON line
// per program on stdout. Nothing is printed when the rejection rate is above 50%.
func Generate(args []string, stdout, stderr io.Writer) int {
	request, err := parseGenerate(args, stderr)
	if err != nil {
		return failed(stderr, GenerateInputInvalid, err)
	}
	run, err := newGenerateRun(request)
	if err != nil {
		return failed(stderr, GenerateInputInvalid, err)
	}
	lines, err := run.produceAll()
	if err != nil && !errors.Is(err, errSlotExhausted) {
		return failed(stderr, exitFor(err), err)
	}
	if run.tally.exceeded() {
		fmt.Fprintf(stderr, "coach-eval generate: %s\n", run.tally.message())
		return GenerateRejectedMost
	}
	for _, output := range lines {
		line, _ := json.Marshal(output)
		fmt.Fprintf(stdout, "%s\n", line)
	}
	return GenerateOK
}

func failed(stderr io.Writer, code int, err error) int {
	fmt.Fprintf(stderr, "coach-eval generate: %v\n", err)
	return code
}

// exitFor maps a failure that is not a rejection: a grader that cannot run is a missing tool.
func exitFor(err error) int {
	if errors.Is(err, evaldetect.ErrGocognitUnavailable) || errors.Is(err, errEvalModuleNotFound) {
		return GenerateInputInvalid
	}
	return GenerateFailed
}
