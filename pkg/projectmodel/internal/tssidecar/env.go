package tssidecar

import "os"

// sanitizedEnv is the analyzer child's environment. When path is non-empty
// it is the child's PATH exactly (production: the probed runtime
// directory). Empty forwards the ambient PATH for tests and fakes. HOME,
// NODE_OPTIONS, HTTP(S)_PROXY, npm_config_*, and every other ambient
// variable are omitted so a leaked loader, proxy, or package-manager
// config cannot influence analysis (AC-RUN-2/AC-RUN-4).
func sanitizedEnv(path string) []string {
	if path != "" {
		return []string{"PATH=" + path}
	}
	var env []string
	if value, ok := os.LookupEnv("PATH"); ok {
		env = append(env, "PATH="+value)
	}
	return env
}
