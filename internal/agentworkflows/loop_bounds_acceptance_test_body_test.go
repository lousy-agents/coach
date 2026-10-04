package agentworkflows

import (
	"regexp"
	"strings"

	. "github.com/onsi/gomega"
)

func body_loopBoundsAcceptanceTest_39(n string, command string) string {
	start := strings.Index(command, "\n"+n+". **")
	Expect(start).To(BeNumerically(">", -1), "step %s not found", n)
	rest := command[start+1:]
	if end := regexp.MustCompile(`\n\d+\. \*\*`).FindStringIndex(rest); end != nil {
		return rest[:end[0]]
	}
	return rest
}

func body_loopBoundsAcceptanceTest_stopsWithAReasonDrawnFromANamedSet_66(step func(n string) string) {
	for _, reason := range []string{"repeated-finding", "agent-failure", "ambiguous-product-decision"} {
		Expect(step("2")).To(ContainSubstring(reason),
			"an untyped stop tells the next reader nothing about what to do")
	}
}
