package main

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"
)

const tsProjectTSConfigJSON = `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"}}`

const tsRealDbFile = "export const Name = 'db';\n"

const tsRealHandlersImportingDB = "import { Name } from \"../db/d\";\n\nexport function use(): string {\n  return Name;\n}\n"

// tsRealHandlersWithoutImport is the negative-control counterpart of
// tsRealHandlersImportingDB, used by no_findings_verdict_acceptance_test.go
// to build a "clean" fixture with no forbidden edge at all.
const tsRealHandlersWithoutImport = "export function use(): string {\n  return 'no import here';\n}\n"

// tsPrismaClientPackageJSON/tsPrismaClientIndexTS mirror
// pkg/projectmodel/ts_sidecar_integration_acceptance_test.go's own
// vendor/prisma-client fixture: a package.json declaring the "@prisma/client"
// name (mirrored into the sidecar's virtual node_modules regardless of its
// real on-disk directory name) plus a PrismaClient class shaped to match
// REACHABILITY_SINK_CLASSES ("user.findMany").
const tsPrismaClientPackageJSON = `{"name":"@prisma/client","main":"index"}`

const tsPrismaClientIndexTS = "export class PrismaClient {\n  user = {\n    findMany(): Promise<unknown[]> {\n      return Promise.resolve([]);\n    },\n  };\n}\n"

// tsServiceNoopFile is a real file under pkg/service/ solely so
// tsLayerBypassRequiredConfigJSON's "service" layer prefix matches at least
// one file in the snapshot -- an unmatched prefix makes
// BuildTypeScriptLayerBypassFromModel treat the required layer as ambiguous
// (see tsLayerBypassRequiredConfigJSON's own doc comment) regardless of any
// bypass witness elsewhere.
const tsServiceNoopFile = "export const noop = 1;\n"

// tsHandlersReachabilityFile is a route handler with a fully resolved call
// path to the pinned Prisma sink, producing one possible_call_reachability
// ProjectFact.
const tsHandlersReachabilityFile = "import { PrismaClient } from \"@prisma/client\";\n\nconst prisma = new PrismaClient();\n\ninterface App {\n  get(path: string, handler: (req: unknown, res: unknown) => void): void;\n}\ndeclare const app: App;\n\nexport async function getUsers(req: unknown, res: unknown): Promise<void> {\n  const users = await prisma.user.findMany();\n  console.log(users, req, res);\n}\napp.get(\"/users\", getUsers);\n"

// tsHandlersLocalGapHelperFile/tsHandlersLocalGapFile reproduce a genuine,
// routine ts_reachability_local_call_not_followed_gap: a route handler
// delegating one hop into an imported local function this sidecar's depth-1
// walk does not itself follow, mirroring
// ts_sidecar_integration_acceptance_test.go's "getUsersCompliant" fixture.
const tsHandlersLocalGapHelperFile = "export function loadStuff(): unknown[] {\n  return [];\n}\n"

const tsHandlersLocalGapFile = "import { loadStuff } from \"./helper\";\n\ninterface App {\n  get(path: string, handler: (req: unknown, res: unknown) => void): void;\n}\ndeclare const app: App;\n\nexport function getStuff(req: unknown, res: unknown): void {\n  const items = loadStuff();\n  console.log(items, req, res);\n}\napp.get(\"/stuff\", getStuff);\n"

// tsLayerBypassRequiredConfigJSON declares handlers/db/service layers with
// service as required_layer, matching tsServiceNoopFile/tsHandlersBypassFile
// -- the TypeScript analog of goLayerBypassPolicyConfigJSON
// (go_project_fixtures_test.go).
const tsLayerBypassRequiredConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]},{"name":"service","prefixes":["pkg/service"]}],"forbidden_imports":[{"from":"handlers","to":"db"}],"required_layer":"service"}`

// tsRootScopeGapTSConfigJSON reproduces SA-280-025's real candidate/analyzed
// root-scope mismatch (mirroring
// ts_sidecar_integration_acceptance_test.go's own "a tsconfig lists a JSON
// file as an explicit root file" spec): resolveJsonModule plus an explicit
// "files" entry accepts package.json into the compiler's Program as a root
// file, but js/semantics/src/project-sidecar/edges.ts's
// extractEdgesFromRootFile only ever visits .ts/.tsx root files, so
// package.json is a candidate the real compiler never actually analyzes.
// "include" is added alongside "files" (TypeScript unions the two) so the
// rest of the project's .ts sources are still discovered and analyzed
// normally, unlike the narrower pkg/projectmodel-level fixture this mirrors.
const tsRootScopeGapTSConfigJSON = `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10","resolveJsonModule":true},"files":["package.json"],"include":["**/*.ts"]}`

// tsHandlersExtraFile is a second real file under pkg/handlers/, alongside
// tsRealHandlersImportingDB, so a root scoped to pkg/handlers (see
// tsNestedRootsScopeConfigJSON) has a distinct, independently-verifiable
// file count from the outer "." root that also contains pkg/db/d.ts.
const tsHandlersExtraFile = "export const extra = 1;\n"

// tsUtilMiscFile lives under a prefix no layer in goLayerPolicyConfigJSON
// declares (only "handlers" and "db" are configured), reproducing AC-5's
// "a file outside every layer" fixture: it must still be counted as an
// ordinary candidate/analyzed file, without spuriously creating or
// expanding any layer's matched set.
const tsUtilMiscFile = "export const misc = 1;\n"

// tsNestedRootsScopeConfigJSON declares two roots where the second
// ("pkg/handlers") nests inside the first ("."), plus a third layer
// ("unused") whose prefix matches no file the fixtures below ever commit --
// reproducing SA-280-005/SA-280-025's independent per-root accounting and
// matched_layers/unmatched_layers split in one fixture.
const tsNestedRootsScopeConfigJSON = `{"schema_version":"1","roots":[".","pkg/handlers"],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]},{"name":"unused","prefixes":["pkg/does-not-exist"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`

func commitNativePackageGapFixture(repo string) {
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
	writeInstalledTypescriptCompilerOnly(repo, "7.0.2")
}

func wrapExecutableWithMarker(exe, marker string) {
	real := exe + ".real"
	Expect(os.Rename(exe, real)).To(Succeed())
	script := fmt.Sprintf("#!/bin/sh\nprintf planted > %q\nexec %q \"$@\"\n", marker, real)
	Expect(os.WriteFile(exe, []byte(script), 0o755)).To(Succeed())
}

func plantCanaryExecutable(exe, marker string) {
	Expect(os.MkdirAll(filepath.Dir(exe), 0o755)).To(Succeed())
	script := fmt.Sprintf("#!/bin/sh\nprintf canary > %q\nexit 1\n", marker)
	Expect(os.WriteFile(exe, []byte(script), 0o755)).To(Succeed())
}

func tsRealCompilerPackageJSON(version string) string {
	return fmt.Sprintf(`{"devDependencies":{"typescript":%q}}`, version)
}

// commitRealTSLayerFixture commits the shared handlers/db/policy fixture
// the real-compiler TypeScript specs build on: an actual value-level import from
// pkg/handlers into pkg/db, forbidden by goLayerPolicyConfigJSON
// (go_project_fixtures_test.go). version is committed into
// package.json so the "project" compiler-resolution origin's manifest/
// installed-version match succeeds once installRealTypescriptCompiler
// copies the matching compiler onto disk.
func commitRealTSLayerFixture(repo, version string) {
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
	commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)
}
