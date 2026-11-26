package evaldetect

import "regexp"

var (
	procBuilt  = regexp.MustCompile(`^/proc/$`)
	procFD     = regexp.MustCompile(`/proc/[^/]+/fd(/|$)`)
	procMemory = regexp.MustCompile(`/proc/[^/]+/mem(/|$)`)
)

// procPatterns are the patterns of a string that names, or begins to build, a /proc path to a descriptor or memory.
// A path being built is the trailing-slash head of a concatenation or template, which a reader cannot finish.
func procPatterns(text string) []string {
	var found []string
	for _, pattern := range []struct {
		name string
		re   *regexp.Regexp
	}{{"proc_path_built", procBuilt}, {"proc_fd", procFD}, {"proc_mem", procMemory}} {
		if pattern.re.MatchString(text) {
			found = append(found, pattern.name)
		}
	}
	return found
}
