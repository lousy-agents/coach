package pkgmanager

import (
	"strings"
)

// bunfigSectionName reads a table header's name. The name runs up to its
// closing ']', not to the end of the line -- real Bun 1.3.11 also honors a
// trailing comment after the header (`[install] # hi`), which
// strings.HasSuffix(line, "]") would reject outright, leaving the section
// unset and a redirect below it unseen. TOML also permits whitespace inside
// the brackets ([ install ]) and a quoted table name (["install"]); both are
// honored the same way by the same Trim.
func bunfigSectionName(line string) string {
	name, _, _ := strings.Cut(line[1:], "]")
	return strings.ToLower(strings.Trim(strings.TrimSpace(name), `"'`))
}

// bunfigRedirectHazard names the redirection it can read directly out of the
// file, walking it as Bun does rather than parsing TOML: a section header
// followed by key/value lines.
func bunfigRedirectHazard(data []byte) string {
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = bunfigSectionName(line)
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = trimConfigValueQuotes(strings.TrimSpace(value))
		switch {
		case section == "install" && key == "registry":
			return "committed bunfig.toml redirects the install registry (registry=" + value + ")"
		case section == "install" && key == "scopes":
			return "committed bunfig.toml redirects scoped install registries (scopes=" + value + ")"
		case section == "install.scopes":
			return "committed bunfig.toml redirects a scoped install registry (" + key + "=" + value + ")"
		}
	}
	return ""
}

// bunfigUnverifiableHazard fails closed on a file this reader cannot claim to
// have understood, rather than on a redirect it recognized.
func bunfigUnverifiableHazard(data []byte) string {
	const unverifiable = "committed bunfig.toml could not be verified to leave package resolution unredirected"

	// Every TOML escape sequence (\uXXXX, \xHH, octal \NNN, and any future
	// form Bun adds) requires a backslash to invoke, in either a quoted
	// table name or a quoted key -- a general check for the mechanism, not
	// an enumeration of its spellings. A legitimate bunfig.toml's forward-
	// slash paths and settings never need one.
	if strings.Contains(string(data), `\`) {
		return unverifiable
	}
	lower := strings.ToLower(string(data))
	for _, keyword := range []string{"install", "registry", "scopes", "cache"} {
		if strings.Contains(lower, keyword) {
			return unverifiable
		}
	}
	return ""
}
