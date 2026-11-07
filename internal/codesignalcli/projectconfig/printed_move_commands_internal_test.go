package projectconfig

// gitAtTheRoot is how a printed move command starts: git run at the repository root, found when the command
// runs, so that it works from any directory.
const gitAtTheRoot = `git -C "$(git rev-parse --show-toplevel)"`

// moveFromTheRoot is the command that makes the directory and then moves the file, with the root found once.
// The arguments are already quoted as they are printed.
func moveFromTheRoot(directory, from, to string) string {
	return `top="$(git rev-parse --show-toplevel)" && mkdir -p "$top"/` + directory + ` && git -C "$top" mv -- ` + from + " " + to
}

// moveWithoutADirectory is the same for a file at the root of the repository, which needs no directory made.
func moveWithoutADirectory(from, to string) string {
	return `top="$(git rev-parse --show-toplevel)" && git -C "$top" mv -- ` + from + " " + to
}
