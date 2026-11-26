package evalgen

const outputInterface = "export interface Output {\n  readonly send: (text: string) => unknown;\n}\n\n"

func (g tsSpec) handlerDeclaration(solution bool) string {
	declaration := "export interface Request {\n  readonly id: string;\n  readonly body: string;\n}\n\n" + outputInterface
	body := "return 0;"
	if solution {
		call := "lower.load(request.id)." + g.field
		if g.cell.Direction == "write" {
			call = "lower.update(request.id, JSON.parse(request.body) as " + g.fieldType(true) + ")." + g.field
		}
		body = "response.send(JSON.stringify(" + call + "));\n  return 200;"
	}
	return declaration + "export function " + g.entryName() + "(request: Request, response: Output): number {\n  " + body + "\n}\n"
}

func (g tsSpec) commandDeclaration(solution bool) string {
	body := "return 0;"
	if solution {
		call := "lower.load(args[0])." + g.field
		if g.cell.Direction == "write" {
			call = "lower.update(args[0], JSON.parse(args[1]) as " + g.fieldType(true) + ")." + g.field
		}
		body = "out.send(JSON.stringify(" + call + "));\n  return 0;"
	}
	return outputInterface + "export function run(args: readonly string[], out: Output): number {\n  " + body + "\n}\n"
}
