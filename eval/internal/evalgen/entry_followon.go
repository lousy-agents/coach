package evalgen

import "fmt"

// followOnDeclaration is the entry once it returns the second field beside the first, through the route the
// first feature already uses. Reading needs only the entry to change; writing also calls the route's function
// for the second field.
func followOnDeclaration(g spec) string {
	fieldType := g.fieldType("lower.")
	switch g.cell.Entry {
	case "function", "method":
		return followOnFunction(g, fieldType)
	case "http_handler":
		return "func " + g.entryName() + "(w http.ResponseWriter, r *http.Request) {\n" + followOnHandlerBody(g, fieldType) + "}\n"
	}
	return "func Run(args []string, out io.Writer) int {\n\t" + followOnCommandBody(g, fieldType) + "\n}\n"
}

func followOnFunction(g spec, fieldType string) string {
	receiver, prefix := "", ""
	if g.cell.Entry == "method" {
		receiver, prefix = "(Service) ", "type Service struct{}\n\n"
	}
	arguments, body := "id string", "record := lower.Load(id)\n\treturn record."+g.field+", record."+g.second
	if g.cell.Direction == "write" {
		arguments = "id string, value " + fieldType + ", other " + fieldType
		body = "first := lower.Update(id, value)\n\tsecond := lower." + g.updateSecond() + "(id, other)\n\treturn first." + g.field + ", second." + g.second
	}
	return prefix + "func " + receiver + g.entryName() + "(" + arguments + ") (" + fieldType + ", " + fieldType + ") {\n\t" + body + "\n}\n"
}

func followOnHandlerBody(g spec, fieldType string) string {
	id := "r.URL.Query().Get(\"id\")"
	pair := "map[string]any{\"" + g.field + "\": %s." + g.field + ", \"" + g.second + "\": %s." + g.second + "}"
	if g.cell.Direction == "read" {
		return "\trecord := lower.Load(" + id + ")\n\t_ = json.NewEncoder(w).Encode(" + fmt.Sprintf(pair, "record", "record") + ")\n"
	}
	return "\tvar body struct {\n\t\t" + g.field + " " + fieldType + "\n\t\t" + g.second + " " + fieldType + "\n\t}\n\t_ = json.NewDecoder(r.Body).Decode(&body)\n" +
		"\tfirst := lower.Update(" + id + ", body." + g.field + ")\n\tsecond := lower." + g.updateSecond() + "(" + id + ", body." + g.second + ")\n" +
		"\t_ = json.NewEncoder(w).Encode(" + fmt.Sprintf(pair, "first", "second") + ")\n"
}

func followOnCommandBody(g spec, fieldType string) string {
	if g.cell.Direction == "read" {
		return "record := lower.Load(args[0])\n\tfmt.Fprintf(out, \"%v %v\\n\", record." + g.field + ", record." + g.second + ")\n\treturn 0"
	}
	return "var first, second " + fieldType + "\n\t_ = json.Unmarshal([]byte(args[1]), &first)\n\t_ = json.Unmarshal([]byte(args[2]), &second)\n" +
		"\tfmt.Fprintf(out, \"%v %v\\n\", lower.Update(args[0], first)." + g.field + ", lower." + g.updateSecond() + "(args[0], second)." + g.second + ")\n\treturn 0"
}
