package evaldetect

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// goModule is a module path and the root-relative directory of its go.mod.
type goModule struct {
	path string
	dir  string
}

// goModules lists every module under root, longest module path first, so an
// import resolves to the innermost module that contains it.
func goModules(root string) ([]goModule, error) {
	var modules []goModule
	err := walkFiles(root, func(name string) bool { return name == "go.mod" }, func(rel string) error {
		modulePath, err := readModulePath(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil && modulePath != "" {
			modules = append(modules, goModule{path: modulePath, dir: path.Dir(rel)})
		}
		return err
	})
	sort.Slice(modules, func(i, j int) bool { return len(modules[i].path) > len(modules[j].path) })
	return modules, err
}

func readModulePath(goMod string) (string, error) {
	file, err := os.Open(goMod)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if fields := strings.Fields(scanner.Text()); len(fields) == 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`), nil
		}
	}
	return "", scanner.Err()
}

// resolveGoImport maps an import path to the root-relative directory it names,
// or reports false for an import outside every module under root.
func resolveGoImport(importPath string, modules []goModule) (string, bool) {
	for _, module := range modules {
		if importPath != module.path && !strings.HasPrefix(importPath, module.path+"/") {
			continue
		}
		return path.Clean(path.Join(module.dir, strings.TrimPrefix(importPath, module.path))), true
	}
	return "", false
}
