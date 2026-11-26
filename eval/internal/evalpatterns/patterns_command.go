package evalpatterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// entryLine is what `coach-eval patterns` prints for one manifest entry.
type entryLine struct {
	Label    string `json:"label"`
	Exit     int    `json:"exit"`
	Identity string `json:"identity,omitempty"`
}

// Patterns runs `coach-eval patterns`: each entry of the manifest is read
// from a fresh isolated checkout of its --clone and its profile written to
// <out>/<label>.profile.json. Every entry is attempted, and the exit is the
// worst entry's: 3 when one does not parse, else 2 when one is refused. A
// command line or manifest that cannot be used exits 2 before any clone is
// read.
func Patterns(args []string, stdout, stderr io.Writer) int {
	parsed, manifest, err := loadPatternsRun(args)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval patterns: %v\n", err)
		return InputInvalid
	}
	exit := Complete
	encoder := json.NewEncoder(stdout)
	for _, reference := range manifest.References {
		identity, err := extractEntry(reference, parsed.clones[reference.Label], parsed.out)
		line := entryLine{Label: reference.Label, Exit: entryExit(err), Identity: identity}
		if err != nil {
			fmt.Fprintf(stderr, "coach-eval patterns: %v\n", err)
		}
		exit = max(exit, line.Exit)
		if err := encoder.Encode(line); err != nil {
			fmt.Fprintf(stderr, "coach-eval patterns: %v\n", err)
			return InputInvalid
		}
	}
	return exit
}

func loadPatternsRun(args []string) (patternsArgs, Manifest, error) {
	parsed, err := parsePatternsArgs(args)
	if err != nil {
		return patternsArgs{}, Manifest{}, err
	}
	manifest, err := LoadManifest(parsed.manifest)
	if err != nil {
		return patternsArgs{}, Manifest{}, err
	}
	if label := parsed.unknownClone(manifest); label != "" {
		return patternsArgs{}, Manifest{}, fmt.Errorf("--clone names %q, which the manifest does not hold", label)
	}
	return parsed, manifest, nil
}

// entryExit is the exit code an entry's error gives: a refusal's own code,
// and 2 for anything else, such as an out directory that cannot be written.
func entryExit(err error) int {
	var refusal Refusal
	switch {
	case err == nil:
		return Complete
	case errors.As(err, &refusal):
		return refusal.Exit
	}
	return InputInvalid
}
