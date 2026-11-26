package validationtasks

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// shellMeta is every character that lets a command line carry text go test
// never receives: a comment, a second command, a substitution, a quote.
const shellMeta = "#;&|$`<>(){}\"'\\\n"

var minutesWord = regexp.MustCompile(`^(\d+)m$`)

// goTestTimeoutMinutes returns the minutes of the -timeout flag that a single
// command passes to go test, whether it runs `go test` itself or
// ./cmd/acceptance-ci-env, which forwards its arguments. A command that
// carries any shell metacharacter is not one command line, and one that
// carries -args hands the test binary flags that can override the timeout, so
// neither names a timeout.
func goTestTimeoutMinutes(command string) (int, bool) {
	words := strings.Fields(command)
	if strings.ContainsAny(command, shellMeta) || slices.Contains(words, "-args") {
		return 0, false
	}
	for i, word := range words {
		switch {
		case word == "go" && i+1 < len(words) && words[i+1] == "test":
			return timeoutFlag(words[i+2:])
		case word == "./cmd/acceptance-ci-env":
			return timeoutFlag(words[i+1:])
		}
	}
	return 0, false
}

func timeoutFlag(args []string) (int, bool) {
	for i, arg := range args {
		value := ""
		switch {
		case arg == "-timeout" && i+1 < len(args):
			value = args[i+1]
		case strings.HasPrefix(arg, "-timeout="):
			value = strings.TrimPrefix(arg, "-timeout=")
		default:
			continue
		}
		if match := minutesWord.FindStringSubmatch(value); match != nil {
			minutes, err := strconv.Atoi(match[1])
			return minutes, err == nil
		}
		return 0, false
	}
	return 0, false
}
