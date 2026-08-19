package greet

// Hello is a workspace-internal export imported by modulea/pkg, used to
// exercise the "internal" ImportEdge.Kind classification for a module whose
// declared path ("moduleab") is dotless and is a longer string that starts
// with another dotless module's full path ("modulea").
var Hello = "hello"
