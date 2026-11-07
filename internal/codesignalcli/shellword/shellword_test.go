package shellword

import "testing"

func TestWordLeavesPlainTextAndQuotesTheRest(t *testing.T) {
	cases := []struct{ text, want string }{
		{"project.json", "project.json"},
		{"policies/p.json", "policies/p.json"},
		{"$(touch PWNED).json", `'$(touch PWNED).json'`},
		{"it's.json", `'it'\''s.json'`},
		{"my policy.json", `'my policy.json'`},
		{"a[1]*.json", `'a[1]*.json'`},
		{"~/p.json", `'~/p.json'`},
		{"~root/p.json", `'~root/p.json'`},
		{"HEAD^", `'HEAD^'`},
		{"a^b.json", `'a^b.json'`},
		{"origin/main@1", "origin/main@1"},
		{"--base=origin/main", "--base=origin/main"},
		{"x,y%z+w:v.json", "x,y%z+w:v.json"},
		{"=lead.json", `'=lead.json'`},
		{"^lead.json", `'^lead.json'`},
		{"@lead.json", "@lead.json"},
		{"", `''`},
	}
	for _, c := range cases {
		if got := Word(c.text); got != c.want {
			t.Errorf("Word(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestPrintableRefusesWhatWouldReachATerminalRaw(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"pol.json", true},
		{"my policy.json", true},
		{"naïve.json", true},
		{"bad\x1bname.json", false},
		{"\x1bname.json", false},
		{"\nname.json", false},
		{"a\nb.json", false},
		{"my\x9bconfig.json", false},
		{"my\u00a0policy.json", true},
		{"my\ue000policy.json", true},
		{"my\u200bpolicy.json", false},
		{"my\u2028policy.json", false},
		{"my\u2029policy.json", false},
		{"my\u202epolicy.json", false},
		{"del\x7fete.json", false},
		{"tab\there.json", false},
	}
	for _, c := range cases {
		if got := Printable(c.text); got != c.want {
			t.Errorf("Printable(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestJoinQuotesEachWordOnly(t *testing.T) {
	got := Join([]string{"coach", "codesignal", "--build-target", "a b", "--base=origin/main"})

	if want := "coach codesignal --build-target 'a b' --base=origin/main"; got != want {
		t.Errorf("Join = %q, want %q", got, want)
	}
}
