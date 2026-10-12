package main

import (
	"os"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/configauthoring"
)

func suggestProjectConfigRequested(args []string) bool {
	for _, arg := range args {
		if suggestFlagRequestsConfig(arg) {
			return true
		}
	}
	return false
}

func suggestFlagRequestsConfig(arg string) bool {
	if arg == "--suggest-project-config" || arg == "-suggest-project-config" {
		return true
	}
	value, ok := suggestProjectConfigFlagValue(arg)
	if !ok {
		return false
	}
	requested, err := strconv.ParseBool(value)
	return err != nil || requested
}

func suggestProjectConfigFlagValue(arg string) (string, bool) {
	for _, prefix := range []string{"--suggest-project-config=", "-suggest-project-config="} {
		if strings.HasPrefix(arg, prefix) {
			return arg[len(prefix):], true
		}
	}
	return "", false
}

func writeSuggestInvalidArguments(stderr *os.File, message string) {
	stderr.Write(configauthoring.InvalidArgumentsSuggestionEnvelope(message))
}
