package evalgen

import (
	"fmt"
	"slices"
	"strings"
)

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
