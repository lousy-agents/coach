package evalgen

import (
	"fmt"
	"slices"
	"strings"
)

// followOnTest is the hidden follow-on test. It asserts the first field and the second, so an entry that breaks
// the first while adding the second fails it. Like the feature test it is its own package.
func followOnTest(g spec) []byte {
	first, second := g.shape.sample, g.secondShape.sample
	if g.cell.Direction == "write" {
		first, second = g.shape.value, g.secondShape.value
	}
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
	b.WriteString("package followon_test\n\nimport (\n")
	for _, path := range imports {
		b.WriteString("\t\"" + path + "\"\n")
	}
	b.WriteString("\n\tentry \"" + g.module + "/" + g.route[0].dir + "\"\n")
	if g.cell.Field == "nested" {
		b.WriteString("\tlower \"" + g.module + "/" + g.route[1].dir + "\"\n")
	}
	b.WriteString(")\n\nfunc TestEntryExposesBothFields(t *testing.T) {\n\twantFirst := " + qualify(first, "lower.") + "\n\twantSecond := " + qualify(second, "lower.") + "\n")
	b.WriteString(followOnCall(g))
	b.WriteString("}\n")
	return formatted(b.String())
}

const compareBoth = "\tif !reflect.DeepEqual(gotFirst, wantFirst) {\n\t\tt.Fatalf(\"first field: got %v, want %v\", gotFirst, wantFirst)\n\t}\n\tif !reflect.DeepEqual(gotSecond, wantSecond) {\n\t\tt.Fatalf(\"second field: got %v, want %v\", gotSecond, wantSecond)\n\t}\n"

func followOnCall(g spec) string {
	arguments := ""
	if g.cell.Direction == "write" {
		arguments = ", wantFirst, wantSecond"
	}
	switch g.cell.Entry {
	case "function":
		return "\tgotFirst, gotSecond := entry." + g.entryName() + "(\"" + recordID + "\"" + arguments + ")\n" + compareBoth
	case "method":
		return "\tgotFirst, gotSecond := entry.Service{}." + g.entryName() + "(\"" + recordID + "\"" + arguments + ")\n" + compareBoth
	case "http_handler":
		return followOnHandlerCall(g)
	}
	args := "\"" + recordID + "\""
	if g.cell.Direction == "write" {
		args += ", `" + g.shape.jsonValue + "`, `" + g.secondShape.jsonValue + "`"
	}
	return fmt.Sprintf("\tvar out bytes.Buffer\n\tcode := entry.Run([]string{%s}, &out)\n\tif code != 0 || out.String() != fmt.Sprintf(\"%%v %%v\\n\", wantFirst, wantSecond) {\n\t\tt.Fatalf(\"code %%d, output %%q\", code, out.String())\n\t}\n", args)
}

func followOnHandlerCall(g spec) string {
	body := "nil"
	if g.cell.Direction == "write" {
		body = "strings.NewReader(`{\"" + g.field + "\":" + g.shape.jsonValue + ",\"" + g.second + "\":" + g.secondShape.jsonValue + "}`)"
	}
	fieldType := g.fieldType("lower.")
	return "\trequest := httptest.NewRequest(http.MethodPost, \"/?id=" + recordID + "\", " + body + ")\n\trecorder := httptest.NewRecorder()\n\tentry." + g.entryName() + "(recorder, request)\n" +
		"\tvar got struct {\n\t\t" + g.field + " " + fieldType + "\n\t\t" + g.second + " " + fieldType + "\n\t}\n" +
		"\tif err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {\n\t\tt.Fatalf(\"response %q: %v\", recorder.Body.String(), err)\n\t}\n" +
		"\tgotFirst, gotSecond := got." + g.field + ", got." + g.second + "\n" + compareBoth
}
