package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen/antipatternfixtures"
	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

func fixturesFor(path, out string, stderr io.Writer) fixturesLine {
	label := strings.TrimSuffix(filepath.Base(path), profileSuffix)
	line := fixturesLine{Label: label, Emitted: []string{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval patterns fixtures: %v\n", err)
		line.Exit = antipatternfixtures.InputInvalid
		return line
	}
	result, err := antipatternfixtures.Generate(raw, evalpatterns.ValidateProfile, filepath.Join(out, label), antipatternfixtures.Templates(), antipatternfixtures.GoVet)
	for _, emitted := range result.Emitted {
		line.Emitted = append(line.Emitted, emitted.ID)
	}
	if len(line.Emitted) == 0 && err == nil {
		fmt.Fprintf(stderr, "coach-eval patterns fixtures: %s: the profile observed no #871 pattern a template reads\n", filepath.Base(path))
	}
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval patterns fixtures: %s: %v\n", filepath.Base(path), err)
		line.Exit = exitOf(err)
	}
	return line
}

func exitOf(err error) int {
	var refusal antipatternfixtures.Refusal
	if errors.As(err, &refusal) {
		return refusal.Exit
	}
	return antipatternfixtures.InputInvalid
}
