package evalharness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

var goTestFunction = regexp.MustCompile(`(?m)^func (Test\w+)\(`)

// testID names a test by its package directory and function: the same function name recurs across packages, and a
// test that is missing from one of them must not be covered by the others.
type testID struct{ directory, name string }

func (id testID) String() string { return id.directory + ": " + id.name }

// requiredTests are the tests an oracle's run must report as passed: the visible tests the manifest lists, and the
// hidden tests of the oracle.
func requiredTests(program evalgen.Program, tree oracleTree) []testID {
	var ids []testID
	for _, visible := range program.Manifest.Template.VisibleTests {
		for _, name := range visible.TestIDs {
			ids = append(ids, testID{path.Dir(visible.Path), name})
		}
	}
	for file, source := range tree.hiddenTests {
		for _, match := range goTestFunction.FindAllStringSubmatch(string(source), -1) {
			ids = append(ids, testID{path.Dir(file), match[1]})
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids
}

// gradeNativeG1 runs go test on an oracle and requires every required test to have reported pass. A test that is
// missing from the report, skipped or failed rejects the program; go test exiting 0 is not enough, because it is
// also what a skipped test and a test that never ran leave behind.
func gradeNativeG1(program evalgen.Program, tree oracleTree) (int, error) {
	directory, err := os.MkdirTemp("", "coach-eval-g1")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(directory)
	if err := evalgen.WriteTree(directory, tree.files); err != nil {
		return 0, err
	}
	actions, output, runErr := goTestReport(directory, moduleNameOf(program))
	required := requiredTests(program, tree)
	var problems []string
	for _, id := range required {
		switch {
		case actions[id]["fail"]:
			problems = append(problems, id.String()+" failed")
		case actions[id]["skip"]:
			problems = append(problems, id.String()+" was skipped")
		case !actions[id]["pass"]:
			problems = append(problems, id.String()+" did not report")
		}
	}
	if len(problems) == 0 && runErr != nil {
		problems = append(problems, fmt.Sprintf("go test: %v", runErr))
	}
	if len(problems) > 0 {
		return 0, &evalgen.StepError{Step: evalgen.StepG1Native, Err: fmt.Errorf("%s oracle: %s\n%s", tree.name, strings.Join(problems, "; "), output)}
	}
	return len(required), nil
}

func moduleNameOf(program evalgen.Program) string {
	first, _, _ := strings.Cut(strings.TrimPrefix(string(program.Tree["go.mod"]), "module "), "\n")
	return first
}

// goTestReport runs go test -json and returns the actions each top-level test reported, by package directory.
func goTestReport(directory, module string) (map[testID]map[string]bool, string, error) {
	command := exec.Command("go", "test", "-json", "./...")
	command.Dir = directory
	command.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	actions := map[testID]map[string]bool{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		var event struct{ Action, Package, Test string }
		if json.Unmarshal([]byte(line), &event) != nil || event.Test == "" || strings.Contains(event.Test, "/") {
			continue
		}
		id := testID{strings.TrimPrefix(strings.TrimPrefix(event.Package, module), "/"), event.Test}
		if id.directory == "" {
			id.directory = "."
		}
		if actions[id] == nil {
			actions[id] = map[string]bool{}
		}
		actions[id][event.Action] = true
	}
	return actions, stderr.String(), err
}
