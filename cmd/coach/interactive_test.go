package main

import "testing"

// Guided authoring and the appended command are two presentations of one offer, and both ask whether a person can
// answer: a controlling terminal, and no request for an unattended run. The decision is pure, so that it is pinned on
// every platform and not only where a terminal can be attached.
func TestAPersonCanAnswerOnlyWithATerminalAndNoRequestForAnUnattendedRun(t *testing.T) {
	cases := []struct {
		name          string
		noInteractive bool
		hasTerminal   bool
		want          bool
	}{
		{"a terminal and no request", false, true, true},
		{"a terminal and a request for an unattended run", true, true, false},
		{"no terminal and no request", false, false, false},
		{"no terminal and a request for an unattended run", true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { requireInteractive(t, c.noInteractive, c.hasTerminal, c.want) })
	}
}

func requireInteractive(t *testing.T, noInteractive, hasTerminal, want bool) {
	t.Helper()
	if got := interactive(noInteractive, hasTerminal); got != want {
		t.Errorf("interactive(noInteractive=%v, hasTerminal=%v) = %v; want %v", noInteractive, hasTerminal, got, want)
	}
}
