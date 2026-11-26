package evaldetect

import (
	"fmt"
	"strconv"
)

// fsDescriptorCalls are the fs functions whose first argument is a descriptor. The Sync ones are also read when
// imported by name and called bare, because a destructured import is the usual way to write them.
var fsDescriptorCalls = map[string]bool{
	"read": true, "write": true, "readFile": true, "writeFile": true, "appendFile": true, "close": true, "fstat": true,
	"ftruncate": true, "fsync": true, "fdatasync": true,
}

// firstInheritedFD is the lowest descriptor number a test can inherit: 0 to 2 are standard input, output and error.
const firstInheritedFD = 3

func tsChannelFindings(rel, source string) ([]Finding, error) {
	if err := refuseJSX(rel); err != nil {
		return nil, err
	}
	tokens, refused := scanTS(source)
	if refused != 0 {
		return nil, fmt.Errorf("%w: %s:%d: a slash the lexer cannot classify", ErrRefused, rel, refused)
	}
	var findings []Finding
	for index, token := range tokens {
		for _, pattern := range tsPatterns(tokens, index) {
			findings = append(findings, channelFinding(rel, pattern, token.line))
		}
	}
	return findings, nil
}

func tsPatterns(tokens []tsToken, index int) []string {
	token := tokens[index]
	switch token.kind {
	case tsString, tsTemplate:
		return procPatterns(token.text)
	case tsIdent:
		if isProcessBinding(tokens, index) {
			return []string{"process_binding"}
		}
		if isInheritedFDCall(tokens, index) {
			return []string{"inherited_fd"}
		}
	}
	return nil
}

func isProcessBinding(tokens []tsToken, index int) bool {
	return tokens[index].text == "process" && index+2 < len(tokens) &&
		tokens[index+1].is(tsPunct, ".") && tokens[index+2].is(tsIdent, "binding")
}

// isInheritedFDCall reports an fs call, or a bare Sync call, whose first argument is a descriptor number of 3 or more.
func isInheritedFDCall(tokens []tsToken, index int) bool {
	if index+2 >= len(tokens) || !tokens[index+1].is(tsPunct, "(") {
		return false
	}
	name := tokens[index].text
	member := index > 0 && tokens[index-1].is(tsPunct, ".")
	if !(member && (fsDescriptorCalls[name] || fsDescriptorCalls[trimSync(name)])) && !(bareSync(name)) {
		return false
	}
	number, err := strconv.Atoi(tokens[index+2].text)
	return err == nil && tokens[index+2].kind == tsOther && number >= firstInheritedFD
}

func trimSync(name string) string {
	if len(name) > 4 && name[len(name)-4:] == "Sync" {
		return name[:len(name)-4]
	}
	return ""
}

func bareSync(name string) bool {
	base := trimSync(name)
	return base != "" && fsDescriptorCalls[base]
}
