package main_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	. "github.com/onsi/gomega"
)

var failedPackage = regexp.MustCompile(`(?m)^FAIL[ \t]+(\S+)`)

func failingPackages(output generateOutcome) []string {
	var packages []string
	for _, match := range failedPackage.FindAllStringSubmatch(output.stdout+output.stderr, -1) {
		if strings.Contains(match[1], "/") {
			packages = append(packages, match[1])
		}
	}
	sort.Strings(packages)
	return packages
}

func manifestOf(program generatedProgram) map[string]any {
	content, err := os.ReadFile(filepath.Join(filepath.Dir(program.Path), "manifest.json"))
	Expect(err).NotTo(HaveOccurred())
	manifest := map[string]any{}
	Expect(json.Unmarshal(content, &manifest)).To(Succeed())
	return manifest
}

func followOnField(manifest map[string]any) string {
	template := manifest["template"].(map[string]any)
	followOn, present := template["follow_on"].(map[string]any)
	Expect(present).To(BeTrue(), "the manifest's template names the follow-on")
	return followOn["field"].(string)
}

func followOnFirstField(manifest map[string]any) string {
	template := manifest["template"].(map[string]any)
	return template["follow_on"].(map[string]any)["first_field"].(string)
}

func entityFieldNames(tree map[string][]byte, entity string) []string {
	for _, content := range tree {
		parsed, err := parser.ParseFile(token.NewFileSet(), "", content, 0)
		if err != nil {
			continue
		}
		if names := structFieldNames(parsed, entity); names != nil {
			return names
		}
	}
	return nil
}

func structFieldNames(file *ast.File, entity string) []string {
	var names []string
	ast.Inspect(file, func(node ast.Node) bool {
		spec, isType := node.(*ast.TypeSpec)
		if !isType || spec.Name.Name != entity {
			return true
		}
		names = fieldNamesOf(spec.Type)
		return false
	})
	return names
}

func fieldNamesOf(expression ast.Expr) []string {
	structure, isStruct := expression.(*ast.StructType)
	if !isStruct {
		return nil
	}
	names := make([]string, 0, len(structure.Fields.List))
	for _, field := range structure.Fields.List {
		names = append(names, field.Names[0].Name)
	}
	return names
}

func entityOf(program generatedProgram) string {
	for _, content := range readTree(program.Path) {
		if match := regexp.MustCompile(`(?m)^type (Account|Order|Patient|Book|Invoice) struct`).FindSubmatch(content); match != nil {
			return string(match[1])
		}
	}
	return ""
}
