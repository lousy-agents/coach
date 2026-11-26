package evalpatterns

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// patternsArgs is a parsed `coach-eval patterns` command line.
type patternsArgs struct {
	manifest, out string
	clones        cloneFlags
}

// cloneFlags maps a manifest label to the local clone given for it with
// --clone LABEL=PATH. The manifest holds no path, so each entry's clone is
// named on the command line.
type cloneFlags map[string]string

func (c cloneFlags) String() string { return "" }

func (c cloneFlags) Set(value string) error {
	label, path, ok := strings.Cut(value, "=")
	if !ok || label == "" || path == "" {
		return fmt.Errorf("--clone %q must be LABEL=PATH", value)
	}
	if _, seen := c[label]; seen {
		return fmt.Errorf("--clone names %q twice", label)
	}
	c[label] = path
	return nil
}

func parsePatternsArgs(args []string) (patternsArgs, error) {
	flags := flag.NewFlagSet("patterns", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	parsed := patternsArgs{clones: cloneFlags{}}
	flags.StringVar(&parsed.manifest, "manifest", "", "the reference manifest")
	flags.StringVar(&parsed.out, "out", "", "the directory the profiles are written to")
	flags.Var(parsed.clones, "clone", "LABEL=PATH, the local clone of one entry")
	if err := flags.Parse(args); err != nil {
		return patternsArgs{}, err
	}
	switch {
	case parsed.manifest == "":
		return patternsArgs{}, errors.New("--manifest is required: the reference manifest")
	case parsed.out == "":
		return patternsArgs{}, errors.New("--out is required: the directory the profiles are written to")
	case flags.NArg() != 0:
		return patternsArgs{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return parsed, nil
}

// unknownClone is the first --clone label the manifest does not hold, or "".
func (parsed patternsArgs) unknownClone(manifest Manifest) string {
	held := map[string]bool{}
	for _, reference := range manifest.References {
		held[reference.Label] = true
	}
	for _, label := range sortedKeysOf(parsed.clones) {
		if !held[label] {
			return label
		}
	}
	return ""
}

func sortedKeysOf(clones cloneFlags) []string {
	counts := map[string]int{}
	for label := range clones {
		counts[label] = 1
	}
	return sortedKeys(counts)
}
