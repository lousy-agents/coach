package modelgateway

import (
	"regexp"
	"strings"
)

var findingRefLine = regexp.MustCompile(`(?m)(?:^|\s)finding_ref:\s*(\S+)`)

func extractFindingRefsFromMessages(msgs []Message) []string {
	var refs []string
	for _, m := range msgs {
		refs = appendUniqueFindingRefs(refs, m.Content)
	}
	return refs
}

func appendUniqueFindingRefs(refs []string, content string) []string {
	for _, match := range findingRefLine.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}
		ref := strings.TrimSpace(match[1])
		if ref == "" || containsFindingRef(refs, ref) {
			continue
		}
		refs = append(refs, ref)
	}
	return refs
}

func containsFindingRef(refs []string, ref string) bool {
	for _, existing := range refs {
		if existing == ref {
			return true
		}
	}
	return false
}
