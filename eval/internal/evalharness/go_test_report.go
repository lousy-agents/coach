package evalharness

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
)

// testID names a test by its package directory and function: the same function name recurs across packages, and a
// test that is missing from one of them must not be covered by the others.
type testID struct{ directory, name string }

func (id testID) String() string { return id.directory + ": " + id.name }

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
