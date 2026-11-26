package evalharness

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var (
	goPackageClause = regexp.MustCompile(`(?m)^package\s+(\w+)`)
	goModuleClause  = regexp.MustCompile(`(?m)^module\s+(\S+)`)
)

func planGoProbe(program, from, to string) (probePlan, error) {
	fromPackage, fromName, err := firstGoPackage(program, from)
	if err != nil {
		return probePlan{}, err
	}
	toPackage, _, err := firstGoPackage(program, to)
	if err != nil {
		return probePlan{}, err
	}
	mod, err := os.ReadFile(filepath.Join(program, "go.mod"))
	if err != nil || !goModuleClause.Match(mod) {
		return probePlan{}, fmt.Errorf("%w: %s: no go.mod module to import through", errHookHealthInput, program)
	}
	module := string(goModuleClause.FindSubmatch(mod)[1])
	return probePlan{
		language: "go",
		rel:      filepath.ToSlash(filepath.Join(fromPackage, probeName+".go")),
		content:  fmt.Sprintf("package %s\n\nimport _ %q\n", fromName, module+"/"+filepath.ToSlash(toPackage)),
	}, nil
}

// firstGoPackage is the first directory under layer, by name, that holds a non-test Go file, and that file's package.
func firstGoPackage(program, layer string) (dir, name string, err error) {
	entries, readErr := os.ReadDir(filepath.Join(program, filepath.FromSlash(layer)))
	if readErr != nil {
		return "", "", fmt.Errorf("%w: %s: no layer directory %s: %v", errHookHealthInput, program, layer, readErr)
	}
	for _, entry := range sortedDirs(entries) {
		candidate := filepath.Join(layer, entry)
		if clause, ok := packageOf(filepath.Join(program, candidate)); ok {
			return candidate, clause, nil
		}
	}
	return "", "", fmt.Errorf("%w: %s: no Go package under %s", errHookHealthInput, program, layer)
}

func packageOf(dir string) (string, bool) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	sort.Strings(files)
	for _, file := range files {
		if content, err := os.ReadFile(file); err == nil && goPackageClause.Match(content) {
			return string(goPackageClause.FindSubmatch(content)[1]), true
		}
	}
	return "", false
}
