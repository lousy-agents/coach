package codesignalcli

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

// TestIsMiseToolVersionInRow pins the frozen mise-origin row's supported
// release (owner decision, 2026-08-31, issue #280): the same calver year as
// this repository's own tested mise pin (mise.toml's min_version, currently
// 2026.9.5). A mise tool version outside that year -- older or newer -- must
// be classified as outside the row.
func TestIsMiseToolVersionInRow(t *testing.T) {
	cases := []struct {
		name    string
		version string
		want    bool
	}{
		{"matches the pinned year, patch zero", "2026.1.0", true},
		{"matches the pinned year, exact tested release", "2026.9.5", true},
		{"matches the pinned year, later patch", "2026.12.0", true},
		{"older calver year is outside the row", "2025.12.1", false},
		{"newer calver year is outside the row", "2027.1.0", false},
		{"empty version is outside the row", "", false},
		{"missing calver separator is outside the row", "2026", false},
		{"non-numeric year is outside the row", "vnext.1.0", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isMiseToolVersionInRow(tc.version); got != tc.want {
				t.Errorf("isMiseToolVersionInRow(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}

// miseTomlMinVersionPattern extracts mise.toml's min_version value, e.g.
// `min_version = "2026.9.5"` -> "2026.9.5".
var miseTomlMinVersionPattern = regexp.MustCompile(`(?m)^min_version\s*=\s*"([^"]+)"`)

// TestMiseToolSupportedCalverYearTracksMiseToml guards against the row's
// year silently drifting from mise.toml's min_version after a bump: it reads
// the repository's actual mise.toml and derives the year from its
// min_version, rather than comparing against a second hardcoded literal, so
// a real mise.toml bump makes this test fail rather than staying green
// alongside the drift it exists to catch.
func TestMiseToolSupportedCalverYearTracksMiseToml(t *testing.T) {
	path := filepath.Join("..", "..", "mise.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	m := miseTomlMinVersionPattern.FindSubmatch(data)
	if m == nil {
		t.Fatalf("%s has no `min_version = \"...\"` line; update this test to match its new format", path)
	}
	year, ok := miseCalverYear(string(m[1]))
	if !ok {
		t.Fatalf("mise.toml min_version %q is not a parseable calver version (want YYYY.M.P)", string(m[1]))
	}
	if miseToolSupportedCalverYear != year {
		t.Errorf("miseToolSupportedCalverYear = %q, want %q (mise.toml min_version's year)", miseToolSupportedCalverYear, year)
	}
}

// TestMiseTomlMinVersionYearExtraction covers the extraction helper's
// failure modes directly (missing/malformed min_version), independent of
// the real mise.toml's current contents.
func TestMiseTomlMinVersionYearExtraction(t *testing.T) {
	cases := []struct {
		name     string
		toml     string
		wantYear string
		wantOK   bool
	}{
		{"pinned calver min_version", `min_version = "2026.9.5"` + "\n", "2026", true},
		{"no min_version line", "[tools]\ngo = \"1.27.1\"\n", "", false},
		{"min_version value has no calver separator", `min_version = "2026"` + "\n", "", false},
		{"min_version value is non-numeric", `min_version = "vnext.1.0"` + "\n", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := miseTomlMinVersionPattern.FindSubmatch([]byte(tc.toml))
			var year string
			var ok bool
			if m != nil {
				year, ok = miseCalverYear(string(m[1]))
			}
			if ok != tc.wantOK || year != tc.wantYear {
				t.Errorf("extraction of %q = (%q, %v), want (%q, %v)", tc.toml, year, ok, tc.wantYear, tc.wantOK)
			}
		})
	}
}

// TestMiseInstallCommandIsExactlyTheFrozenTemplate pins AC-10: the install
// command is the parent epic's frozen template, not a value selected during
// coding.
func TestMiseInstallCommandIsExactlyTheFrozenTemplate(t *testing.T) {
	got := miseInstallCommand("7.0.2")
	want := "mise install npm:typescript@7.0.2"
	if got != want {
		t.Errorf("miseInstallCommand(%q) = %q, want %q", "7.0.2", got, want)
	}
}

// TestMiseWhereCommandIsExactlyTheFrozenTemplate pins the frozen row's
// second command, run after install to locate the resulting package.
func TestMiseWhereCommandIsExactlyTheFrozenTemplate(t *testing.T) {
	if miseWhereCommand != "mise where" {
		t.Errorf("miseWhereCommand = %q, want %q", miseWhereCommand, "mise where")
	}
}

// TestMiseBackingNpmSuppressionFlagMatchesTheNpmRow pins that the mise
// npm-backend row reuses the npm adapter row's own suppression mechanism
// (SA-280-009/SA-280-012) rather than a mise-specific flag invented
// separately, since mise installs npm:typescript via npm's own resolution.
func TestMiseBackingNpmSuppressionFlagMatchesTheNpmRow(t *testing.T) {
	if miseBackingNpmSuppressionFlag != "--ignore-scripts" {
		t.Errorf("miseBackingNpmSuppressionFlag = %q, want %q", miseBackingNpmSuppressionFlag, "--ignore-scripts")
	}
}

// TestMiseInstallCommandTakesExactlyOneVersionParameter pins AC-14: the
// row's install target names exactly one version, never a secondary or
// fallback version. It asserts on miseInstallCommand's own reflected
// signature -- an oracle independent of the template literal pinned by
// TestMiseInstallCommandIsExactlyTheFrozenTemplate -- so a future signature
// change to accept a collection of versions (e.g. []string) fails this test
// directly rather than passing because both sides of a comparison moved
// together.
func TestMiseInstallCommandTakesExactlyOneVersionParameter(t *testing.T) {
	fnType := reflect.TypeOf(miseInstallCommand)
	if fnType.NumIn() != 1 {
		t.Fatalf("miseInstallCommand has %d parameters, want exactly 1 (the single resolved version, never a secondary/fallback)", fnType.NumIn())
	}
	if got := fnType.In(0).Kind(); got != reflect.String {
		t.Errorf("miseInstallCommand's parameter kind = %v, want %v (a single version, not a slice/collection)", got, reflect.String)
	}
}
