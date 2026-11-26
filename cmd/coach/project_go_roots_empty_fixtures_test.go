package main

const (
	typescriptManifest       = "{\"name\":\"fixture\",\"private\":true}\n"
	typescriptConfig         = "{\"compilerOptions\":{\"strict\":true}}\n"
	typescriptDatabaseModule = "export const Name = \"db\";\n"
	typescriptHandlerModule  = "export const Use = () => \"\";\n"
	typescriptHandlerEdited  = "import { Name } from \"../db/d\";\nexport const Use = () => Name;\n"

	goRootsEmptyPolicyUnderServices = `{"schema_version":"1","roots":["services"],"layers":[{"name":"handlers","prefixes":["services/handlers"]},{"name":"db","prefixes":["services/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`
)

const rootsEmptyDiagnosticCode = "project_roots_empty"

func typescriptOnlyRepo() string {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", typescriptManifest)
	commitFile(repo, "tsconfig.json", typescriptConfig)
	commitFile(repo, "pkg/db/d.ts", typescriptDatabaseModule)
	commitFile(repo, "pkg/handlers/h.ts", typescriptHandlerModule)
	commitFile(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)
	return repo
}
