package projectconfig

// directoryEntries lists the names directly under a directory, the empty string being the root. A directory that is
// not there has none.
type directoryEntries func(directory string) ([]string, error)
