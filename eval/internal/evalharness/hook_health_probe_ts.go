package evalharness

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

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
