package evalgen

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
