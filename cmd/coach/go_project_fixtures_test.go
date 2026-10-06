package main

const goLayerPolicyConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`

const goModuleFile = "module example.com/app\n\ngo 1.25\n"

const dbPackageFile = "package db\n\n// Name is a placeholder export used by the layer-violation fixtures.\nvar Name = \"db\"\n"

// The import sits on line 3 (0-based row 2).
const handlersImportingDB = "package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n"

const handlersWithoutImport = "package handlers\n\nfunc Use() string {\n\treturn \"\"\n}\n"

// Reaches structure.constructor_density's per-file density gate
// (densityGateThreshold == 2 in registry.go); paired with
// handlersImportingDB, exercises severity-based signal sort ordering.
const modelFileWithTwoConstructors = "package model\n\ntype A struct{}\n\ntype B struct{}\n\nfunc NewA() *A {\n\treturn &A{}\n}\n\nfunc NewB() *B {\n\treturn &B{}\n}\n"

// No forbidden_imports, so a layer_bypass finding is never accompanied by
// an unrelated layer_violation finding.
const goLayerBypassPolicyConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"service","prefixes":["pkg/service"]},{"name":"db","prefixes":["pkg/db"]}],"required_layer":"service"}`
