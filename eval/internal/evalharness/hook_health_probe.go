package evalharness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const probeName = "zz_probe"

// probePlan is the one file that makes a program break its own policy: written uncommitted, it must make the hook
// block with architecture.layer_violation at its path.
type probePlan struct {
	language string
	rel      string
	content  string
}

type probePolicy struct {
	Language string `json:"language"`
	Layers   []struct {
		Name     string   `json:"name"`
		Prefixes []string `json:"prefixes"`
	} `json:"layers"`
	ForbiddenImports []struct{ From, To string } `json:"forbidden_imports"`
}

var (
	goPackageClause = regexp.MustCompile(`(?m)^package\s+(\w+)`)
	goModuleClause  = regexp.MustCompile(`(?m)^module\s+(\S+)`)
)

// planProbe reads the program's own policy for its first forbidden edge and names a file in the from layer that
// imports a package of the to layer.
func planProbe(program string) (probePlan, error) {
	policy, err := readProbePolicy(program)
	if err != nil {
		return probePlan{}, err
	}
	if len(policy.ForbiddenImports) == 0 {
		return probePlan{}, fmt.Errorf("%w: %s: the policy names no forbidden import to probe", errHookHealthInput, program)
	}
	edge := policy.ForbiddenImports[0]
	from, to := layerDir(policy, edge.From), layerDir(policy, edge.To)
	if from == "" || to == "" {
		return probePlan{}, fmt.Errorf("%w: %s: the forbidden import %s -> %s names a layer the policy does not define", errHookHealthInput, program, edge.From, edge.To)
	}
	if policy.Language == "typescript" {
		return planTypeScriptProbe(program, from, to)
	}
	return planGoProbe(program, from, to)
}

func readProbePolicy(program string) (probePolicy, error) {
	var policy probePolicy
	content, err := os.ReadFile(filepath.Join(program, ".lousy-agents", "coach.json"))
	if err != nil {
		return policy, fmt.Errorf("%w: %s: no policy to read: %v", errHookHealthInput, program, err)
	}
	if err := json.Unmarshal(content, &policy); err != nil {
		return policy, fmt.Errorf("%w: %s: the policy is not valid JSON: %v", errHookHealthInput, program, err)
	}
	return policy, nil
}

func layerDir(policy probePolicy, name string) string {
	for _, layer := range policy.Layers {
		if layer.Name == name && len(layer.Prefixes) > 0 {
			return layer.Prefixes[0]
		}
	}
	return ""
}

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

func planTypeScriptProbe(program, from, to string) (probePlan, error) {
	toFile, err := firstTypeScriptFile(program, to)
	if err != nil {
		return probePlan{}, err
	}
	if _, err := firstTypeScriptFile(program, from); err != nil {
		return probePlan{}, err
	}
	relative, err := filepath.Rel(filepath.FromSlash(from), toFile)
	if err != nil {
		return probePlan{}, err
	}
	return probePlan{
		language: "typescript",
		rel:      filepath.ToSlash(filepath.Join(from, probeName+".ts")),
		content:  fmt.Sprintf("import %q;\n", filepath.ToSlash(relative)),
	}, nil
}

func firstTypeScriptFile(program, layer string) (string, error) {
	files, _ := filepath.Glob(filepath.Join(program, filepath.FromSlash(layer), "*.ts"))
	sort.Strings(files)
	for _, file := range files {
		if !strings.HasSuffix(file, ".test.ts") {
			return filepath.Join(layer, filepath.Base(file)), nil
		}
	}
	return "", fmt.Errorf("%w: %s: no TypeScript source under %s", errHookHealthInput, program, layer)
}
