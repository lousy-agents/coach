package evalharness

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

var goTestFunction = regexp.MustCompile(`(?m)^func (Test\w+)\(`)

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
