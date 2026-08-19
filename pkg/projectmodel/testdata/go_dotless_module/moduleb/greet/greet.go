package greet

// Hello only exists so this directory has a Go file and therefore appears in
// packageDirs: it is the package directory moduleb's declared path
// ("moduleab") maps an import of "moduleab/greet" to, making "moduleab" a
// genuine candidate that the longest-match tiebreak in classifyGoImport must
// reject in favor of moduleb/sub's declared path ("moduleab/greet").
var Hello = "hello"
