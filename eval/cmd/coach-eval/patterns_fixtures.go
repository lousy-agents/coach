package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen/antipatternfixtures"
	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

const profileSuffix = ".profile.json"

// patterns runs `coach-eval patterns`, or `coach-eval patterns fixtures`.
func patterns(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "fixtures" {
		return patternsFixtures(args[1:], stdout, stderr)
	}
	return evalpatterns.Patterns(args, stdout, stderr)
}

// fixturesLine is what `coach-eval patterns fixtures` prints for one profile.
type fixturesLine struct {
	Label   string   `json:"label"`
	Exit    int      `json:"exit"`
	Emitted []string `json:"emitted"`
}

// patternsFixtures writes, for each <label>.profile.json in --profiles, the
// fixtures its profile yields into <out>/<label>. Each profile is checked
// against the committed profile schema first. The exit is the worst
// profile's.
func patternsFixtures(args []string, stdout, stderr io.Writer) int {
	profiles, out, err := parseFixturesArgs(args)
	if err == nil {
		profiles, err = profileFiles(profiles)
	}
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval patterns fixtures: %v\n", err)
		return antipatternfixtures.InputInvalid
	}
	exit := 0
	encoder := json.NewEncoder(stdout)
	for _, path := range profilesList(profiles) {
		line := fixturesFor(path, out, stderr)
		exit = max(exit, line.Exit)
		if err := encoder.Encode(line); err != nil {
			return antipatternfixtures.InputInvalid
		}
	}
	return exit
}

func parseFixturesArgs(args []string) (profiles, out string, err error) {
	flags := flag.NewFlagSet("patterns fixtures", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&profiles, "profiles", "", "the directory of <label>.profile.json files")
	flags.StringVar(&out, "out", "", "the directory each label's fixtures are written under")
	if err := flags.Parse(args); err != nil {
		return "", "", err
	}
	switch {
	case profiles == "":
		return "", "", errors.New("--profiles is required: the directory of profiles")
	case out == "":
		return "", "", errors.New("--out is required: the directory the fixtures are written under")
	case flags.NArg() != 0:
		return "", "", fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return profiles, out, nil
}

// profileFiles is profiles, refused when it holds no profile.
func profileFiles(profiles string) (string, error) {
	if len(profilesList(profiles)) == 0 {
		return "", fmt.Errorf("no profile (*%s) in %s", profileSuffix, profiles)
	}
	return profiles, nil
}

func profilesList(profiles string) []string {
	paths, _ := filepath.Glob(filepath.Join(profiles, "*"+profileSuffix))
	sort.Strings(paths)
	return paths
}

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
