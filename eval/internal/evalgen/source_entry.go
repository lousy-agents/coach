package evalgen

import (
	"fmt"
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

// entrySource is the entry package: its stub returns the zero value, and the clean oracle forwards to the
// next package on the route. Both declare Describe so the visible tests do not depend on the entry.
func entrySource(g spec, solution bool) []byte {
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
	b.WriteString(entryDeclaration(g, solution))
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

func entryDeclaration(g spec, solution bool) string {
	fieldType := g.fieldType("lower.")
	call := "lower.Load(id)." + g.field
	arguments := "id string"
	if g.cell.Direction == "write" {
		call = "lower.Update(id, value)." + g.field
		arguments = "id string, value " + fieldType
	}
	switch g.cell.Entry {
	case "function", "method":
		receiver, prefix := "", ""
		if g.cell.Entry == "method" {
			receiver, prefix = "(Service) ", "type Service struct{}\n\n"
		}
		body := "var result " + fieldType + "\n\treturn result"
		if solution {
			body = "return " + call
		}
		return prefix + "func " + receiver + g.entryName() + "(" + arguments + ") " + fieldType + " {\n\t" + body + "\n}\n"
	case "http_handler":
		body := ""
		if solution {
			body = handlerBody(g, fieldType)
		}
		return "func " + g.entryName() + "(w http.ResponseWriter, r *http.Request) {\n" + body + "}\n"
	}
	body := "return 0"
	if solution {
		body = commandBody(g, fieldType)
	}
	return "func Run(args []string, out io.Writer) int {\n\t" + body + "\n}\n"
}

func handlerBody(g spec, fieldType string) string {
	id := "r.URL.Query().Get(\"id\")"
	if g.cell.Direction == "read" {
		return "\t_ = json.NewEncoder(w).Encode(lower.Load(" + id + ")." + g.field + ")\n"
	}
	return "\tvar value " + fieldType + "\n\t_ = json.NewDecoder(r.Body).Decode(&value)\n\t_ = json.NewEncoder(w).Encode(lower.Update(" + id + ", value)." + g.field + ")\n"
}

func commandBody(g spec, fieldType string) string {
	if g.cell.Direction == "read" {
		return "fmt.Fprintf(out, \"%v\\n\", lower.Load(args[0])." + g.field + ")\n\treturn 0"
	}
	return "var value " + fieldType + "\n\t_ = json.Unmarshal([]byte(args[1]), &value)\n\tfmt.Fprintf(out, \"%v\\n\", lower.Update(args[0], value)." + g.field + ")\n\treturn 0"
}

// featureTest is the hidden FAIL_TO_PASS test. It lives in its own package, so the stub still compiles.
func featureTest(g spec) []byte {
	value := g.shape.sample
	if g.cell.Direction == "write" {
		value = g.shape.value
	}
	expected := qualify(value, "lower.")
	imports := []string{"reflect", "testing"}
	switch g.cell.Entry {
	case "http_handler":
		imports = append(imports, "encoding/json", "net/http", "net/http/httptest")
		if g.cell.Direction == "write" {
			imports = append(imports, "strings")
		}
	case "command":
		imports = []string{"bytes", "fmt", "testing"}
	}
	slices.Sort(imports)
	var b strings.Builder
	b.WriteString("package feature_test\n\nimport (\n")
	for _, path := range imports {
		b.WriteString("\t\"" + path + "\"\n")
	}
	b.WriteString("\n\tentry \"" + g.module + "/" + g.route[0].dir + "\"\n")
	if g.cell.Field == "nested" {
		b.WriteString("\tlower \"" + g.module + "/" + g.route[1].dir + "\"\n")
	}
	b.WriteString(")\n\nfunc TestEntryExposesTheField(t *testing.T) {\n\twant := " + expected + "\n")
	b.WriteString(featureCall(g))
	b.WriteString("}\n")
	return formatted(b.String())
}

func featureCall(g spec) string {
	argument := ""
	if g.cell.Direction == "write" {
		argument = ", want"
	}
	switch g.cell.Entry {
	case "function":
		return "\tgot := entry." + g.entryName() + "(\"" + recordID + "\"" + argument + ")\n" + compare
	case "method":
		return "\tgot := entry.Service{}." + g.entryName() + "(\"" + recordID + "\"" + argument + ")\n" + compare
	case "http_handler":
		body := "nil"
		if g.cell.Direction == "write" {
			body = "strings.NewReader(`" + g.shape.jsonValue + "`)"
		}
		return "\trequest := httptest.NewRequest(http.MethodPost, \"/?id=" + recordID + "\", " + body + ")\n\trecorder := httptest.NewRecorder()\n\tentry." + g.entryName() + "(recorder, request)\n\tvar got " + g.fieldType("lower.") + "\n\tif err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {\n\t\tt.Fatalf(\"response %q: %v\", recorder.Body.String(), err)\n\t}\n" + compare
	}
	arguments := "\"" + recordID + "\""
	if g.cell.Direction == "write" {
		arguments += ", `" + g.shape.jsonValue + "`"
	}
	return fmt.Sprintf("\tvar out bytes.Buffer\n\tcode := entry.Run([]string{%s}, &out)\n\tif code != 0 || out.String() != fmt.Sprintf(\"%%v\\n\", want) {\n\t\tt.Fatalf(\"code %%d, output %%q\", code, out.String())\n\t}\n", arguments)
}

const compare = "\tif !reflect.DeepEqual(got, want) {\n\t\tt.Fatalf(\"got %v, want %v\", got, want)\n\t}\n"
