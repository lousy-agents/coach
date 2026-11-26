package evalgen

import (
	"slices"
	"strings"
)

func (g spec) fieldType(qualifier string) string { return qualify(g.shape.typeName, qualifier) }

func (g spec) entryName() string {
	if g.cell.Entry == "command" {
		return "Run"
	}
	if g.cell.Direction == "read" {
		return "Get" + g.field
	}
	return "Set" + g.field
}

// stage says how far the entry package has got: the stub returns the zero value, the first feature forwards
// to the next package on the route, and the follow-on returns the second field beside the first.
type stage int

const (
	stubStage stage = iota
	firstStage
	followOnStage
)

// entrySource is the entry package at a stage. Every stage declares Describe so the visible tests do not
// depend on the entry.
func entrySource(g spec, at stage) []byte {
	solution := at != stubStage
	lower := g.route[1]
	std := []string{}
	switch g.cell.Entry {
	case "http_handler":
		std = append(std, "net/http")
		if solution {
			std = append(std, "encoding/json")
		}
	case "command":
		std = append(std, "io")
		if solution {
			std = append(std, "fmt")
		}
		if solution && g.cell.Direction == "write" {
			std = append(std, "encoding/json")
		}
	}
	slices.Sort(std)
	var b strings.Builder
	b.WriteString("package " + g.route[0].name + "\n\nimport (\n")
	for _, path := range std {
		b.WriteString("\t\"" + path + "\"\n")
	}
	importsLower := solution || stubNamesLowerType(g)
	if importsLower {
		b.WriteString("\n\tlower \"" + g.module + "/" + lower.dir + "\"\n")
	}
	b.WriteString(")\n\nfunc Describe() string {\n\treturn \"" + g.route[0].layer + "/" + g.route[0].name + "\"\n}\n\n")
	b.WriteString(entryDeclaration(g, at))
	source := b.String()
	if len(std) == 0 && !importsLower {
		source = strings.Replace(source, "import (\n)\n\n", "", 1)
	}
	return formatted(source)
}

// stubNamesLowerType is true when the stub's signature or body mentions a type the next package owns, which
// only a returning function or method does.
func stubNamesLowerType(g spec) bool {
	return g.cell.Field == "nested" && (g.cell.Entry == "function" || g.cell.Entry == "method")
}
