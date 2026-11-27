package main

import (
	"encoding/json"
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
	raw, err := labelledProfile(path, label)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval patterns fixtures: %s: %v\n", filepath.Base(path), err)
		line.Exit = antipatternfixtures.InputInvalid
		return line
	}
	result, err := antipatternfixtures.Generate(raw, evalpatterns.ValidateSealedProfile, out, label, antipatternfixtures.Templates(), antipatternfixtures.GoVet)
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

// labelledProfile reads the profile at path and requires its file to be
// named after the label it holds. Generate then validates the profile, whose
// schema holds the label to a path segment, and clears only the label's own
// directory under --out.
func labelledProfile(path, label string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var held struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(raw, &held); err != nil {
		return nil, err
	}
	if held.Label != label {
		return nil, fmt.Errorf("the file holds label %q, so it must be named %s%s", held.Label, held.Label, profileSuffix)
	}
	return raw, nil
}
