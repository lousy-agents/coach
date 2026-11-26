package antipatternfixtures

import (
	"fmt"
	"os"
	"strings"
)

// Result is what a run emitted.
type Result struct {
	Emitted []Emitted
}

// Emitted is one fixture written, and the value it is scaled to.
type Emitted struct {
	ID    string
	Value int
}

// Generate writes into root/label, which it first clears, one fixture for
// each template whose shape the profile observed. A profile that fails
// validate or is of another version, or a label that is not one path
// segment, is refused with exit 2 before anything is removed. A fixture
// whose variant fails vet is left out, the others are written, and the run
// exits 3 naming it.
func Generate(raw []byte, validate Validator, root, label string, templates []Template, vet Vetter) (Result, error) {
	read, err := readProfile(raw, validate)
	if err != nil {
		return Result{}, err
	}
	out, err := labelDir(root, label)
	if err != nil {
		return Result{}, err
	}
	if err := os.RemoveAll(out); err != nil {
		return Result{}, refuse(InputInvalid, fmt.Sprintf("the output directory cannot be cleared: %v", err))
	}
	var result Result
	var failed []string
	for _, template := range templates {
		seen, ok := template.observe(read)
		if !ok {
			continue
		}
		if err := emit(template, seen, read.Identity, out, vet); err != nil {
			failed = append(failed, template.ID+": "+err.Error())
			continue
		}
		result.Emitted = append(result.Emitted, Emitted{ID: template.ID, Value: seen.value})
	}
	if len(failed) > 0 {
		return result, refuse(AnalysisFailed, "left out, a variant fails go vet: "+strings.Join(failed, "; "))
	}
	return result, nil
}

func emit(template Template, seen observation, identity, out string, vet Vetter) error {
	bad, good := template.Bad(seen.value), template.Good(seen.value)
	for _, variant := range []map[string]string{bad, good} {
		if err := vetVariant(variant, vet); err != nil {
			return err
		}
	}
	return writeFixture(out, template, seen, identity, bad, good)
}
