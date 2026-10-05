package claudehooks

import (
	"strings"
)

type sigverdictRegistrationMatchersS0 struct {
	matchers *[]string
	reg      struct {
		Matcher string "json:\"matcher\""

		Hooks []struct {
			Args []string "json:\"args\""
		} "json:\"hooks\""
	}
}

func (sigRecv *sigverdictRegistrationMatchersS0) call() {
	for _, h := range sigRecv.reg.Hooks {
		if strings.Contains(strings.Join(h.Args, " "), "verify-review-verdict.sh") {
			*sigRecv.matchers = append(*sigRecv.matchers, sigRecv.reg.Matcher)
		}
	}
}
