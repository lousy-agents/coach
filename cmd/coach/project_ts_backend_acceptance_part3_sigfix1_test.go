package main

import "strings"

type sigpathValueFromEnvironS2 struct {
	part string
}

func (sigRecv *sigpathValueFromEnvironS2) call() (string, bool) {

	if i := strings.Index(sigRecv.part, "PATH="); i >= 0 {
		rest := sigRecv.part[i+5:]
		if j := strings.IndexAny(rest, " \t"); j >= 0 {
			return rest[:j], true
		}
		return rest, true
	}
	return "", false
}
