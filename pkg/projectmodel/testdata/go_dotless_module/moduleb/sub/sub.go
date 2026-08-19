package sub

// Hello only exists so this directory has a Go file and therefore appears in
// packageDirs: this module's declared path ("moduleab/greet") is a strict,
// slash-bounded extension of moduleb's declared path ("moduleab"), so an
// import of "moduleab/greet" genuinely matches both modules and the
// longest-match tiebreak in classifyGoImport decides between them.
var Hello = "hello"
