package evalgen

import (
	"os"
	"path"
	"strings"
)

// Validate runs the native checks on a program: the stub compiles and vets, the visible tests pass on it, the
// hidden tests fail on it in their own package only, the clean oracle passes everything, and the follow-on slot
// repeats the same three steps on top of the clean oracle. Nothing here
// reads Coach output.
func Validate(p Program) error {
	work, err := os.MkdirTemp("", "coach-eval-validate")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	steps := []validationStep{
		{id: StepNative, name: "stub", files: p.Tree, commands: [][]string{{"build", "./..."}, {"vet", "./..."}, {"test", "./..."}}},
		{id: StepNative, name: "hidden tests on the stub", files: p.Tests, fails: []string{p.moduleName() + "/tests/feature"}},
		{id: StepDeleteBaseline, name: "delete baseline", remove: p.entryFile(), target: "./tests/...", fails: []string{p.moduleName() + "/tests/feature"}},
		{id: StepNative, name: "clean oracle", files: p.Solution, commands: [][]string{{"vet", "./..."}, {"test", "./..."}}},
		{id: StepFollowOnStub, name: "follow-on stub", drop: followOnDrops, files: p.FollowOnTests, fails: []string{p.moduleName() + "/tests/followon"}},
		{id: StepFollowOnDeleteBaseline, name: "follow-on delete baseline", remove: p.entryFile(), target: "./tests/followon/...", fails: []string{p.moduleName() + "/tests/followon"}},
		{id: StepFollowOnOracle, name: "follow-on oracle", files: p.FollowOnSolution, commands: [][]string{{"vet", "./..."}, {"test", "./..."}}},
	}
	for _, step := range steps {
		if err := step.run(work); err != nil {
			return &StepError{Step: step.id, Err: err}
		}
	}
	return nil
}

// entryFile is the source file of the entry package, which the delete baseline removes.
func (p Program) entryFile() string {
	entry := p.Manifest.Template.APISurface[0].Entry
	dir := entry[:strings.LastIndex(entry, ".")]
	return dir + "/" + path.Base(dir) + ".go"
}

func (p Program) moduleName() string {
	first, _, _ := strings.Cut(strings.TrimPrefix(string(p.Tree["go.mod"]), "module "), "\n")
	return first
}
