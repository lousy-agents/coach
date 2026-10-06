// Command coach is the composition-root CLI for the coach project.
package main

import (
	"os"
)

// version is overridden via -ldflags at release; a local build reports "dev".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
