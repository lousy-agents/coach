package main

import "testing"

func TestSeeAllCommandDropsNarrowingFlagsInEverySpelling(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"separate values", []string{"--baseline", "--min-severity", "high", "--top", "2"}, "coach codesignal --baseline"},
		{"inline values", []string{"--baseline", "--min-severity=high", "--top=2"}, "coach codesignal --baseline"},
		{"single dash", []string{"-baseline", "-min-severity", "high", "-top=2"}, "coach codesignal -baseline"},
		{"other flags keep their order", []string{"--top", "1", "--base", "origin/main", "--scope", "all"}, "coach codesignal --base origin/main --scope all"},
		{"a value that looks like a narrowing flag stays with its flag", []string{"--build-target", "--top", "--top", "3"}, "coach codesignal --build-target --top"},
		{"quotes a space", []string{"--project-config", "my config.json", "--top", "1"}, "coach codesignal --project-config 'my config.json'"},
		{"quotes an apostrophe", []string{"--build-target", "it's", "--top", "1"}, `coach codesignal --build-target 'it'\''s'`},
		{"quotes an empty value", []string{"--build-target", "", "--top", "1"}, "coach codesignal --build-target ''"},
		{"quotes a leading equals that zsh would expand to a path", []string{"--base", "=ls", "--top", "1"}, "coach codesignal --base '=ls'"},
		{"quotes a leading tilde", []string{"--base", "~/x", "--top", "1"}, "coach codesignal --base '~/x'"},
		{"leaves an inner equals unquoted", []string{"--base", "a=b", "--top", "1"}, "coach codesignal --base a=b"},
		{"keeps accented letters", []string{"--build-target", "caf\u00e9", "--top", "1"}, "coach codesignal --build-target 'caf\u00e9'"},
		{"keeps CJK letters", []string{"--build-target", "\u65e5\u672c\u8a9e", "--top", "1"}, "coach codesignal --build-target '\u65e5\u672c\u8a9e'"},
		{"keeps a no-break space", []string{"--build-target", "a\u00a0b", "--top", "1"}, "coach codesignal --build-target 'a\u00a0b'"},
		{"no arguments", nil, "coach codesignal"},
	}
	for _, tc := range cases {
		if got := seeAllCommand(tc.args); got != tc.want {
			t.Errorf("%s: seeAllCommand(%q) = %q, want %q", tc.name, tc.args, got, tc.want)
		}
	}
}

func TestSeeAllCommandOmitsTheCommandWhenAnyWordHoldsAControlCharacter(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"newline", []string{"--project-config", "a\nb", "--top", "1"}},
		{"escape", []string{"--build-target", "\x1b[2J", "--top", "1"}},
		{"carriage return", []string{"--base", "a\rb", "--top", "1"}},
		{"tab", []string{"--base", "a\tb", "--top", "1"}},
		{"delete", []string{"--base", "a\x7fb", "--top", "1"}},
		{"nul", []string{"--base", "a\x00b", "--top", "1"}},
		{"c1 control", []string{"--base", "a\u0080b", "--top", "1"}},
		{"8-bit csi", []string{"--base", "a\u009bb", "--top", "1"}},
		{"last c1 control", []string{"--base", "a\u009fb", "--top", "1"}},
		{"invalid utf-8 byte", []string{"--base", "a\x9bb", "--top", "1"}},
		{"truncated utf-8 sequence", []string{"--base", "a\xe2\x80", "--top", "1"}},
		{"right-to-left override", []string{"--base", "a\u202eb", "--top", "1"}},
		{"left-to-right isolate", []string{"--base", "a\u2066b", "--top", "1"}},
		{"right-to-left mark", []string{"--base", "a\u200fb", "--top", "1"}},
		{"zero-width space", []string{"--base", "a\u200bb", "--top", "1"}},
		{"word joiner", []string{"--base", "a\u2060b", "--top", "1"}},
		{"byte order mark", []string{"--base", "a\ufeffb", "--top", "1"}},
		{"soft hyphen", []string{"--base", "a\u00adb", "--top", "1"}},
		{"tag character", []string{"--base", "a\U000e0041b", "--top", "1"}},
		{"line separator", []string{"--base", "a\u2028b", "--top", "1"}},
		{"paragraph separator", []string{"--base", "a\u2029b", "--top", "1"}},
		{"a positional word", []string{"--top", "1", "--baseline", "--", "x\ny"}},
	}
	for _, tc := range cases {
		if got := seeAllCommand(tc.args); got != "" {
			t.Errorf("%s: seeAllCommand(%q) = %q, want no command", tc.name, tc.args, got)
		}
	}
}

func TestSeeAllCommandKeepsPrintableNonASCIIWords(t *testing.T) {
	got := seeAllCommand([]string{"--build-target", "pkg/\u00fcn\u00ef", "--top", "1"})

	if want := "coach codesignal --build-target 'pkg/\u00fcn\u00ef'"; got != want {
		t.Errorf("seeAllCommand = %q, want %q", got, want)
	}
}
