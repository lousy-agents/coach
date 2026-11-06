package projectconfig

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckExactNamesRefusesASpellingThatDiffersOnlyByCase(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  string
	}{
		{"the file", []string{"cfg/P.JSON"}, `"P.JSON"`},
		{"a parent directory", []string{"CFG/p.json"}, `"CFG"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertCaseDifferenceRefused(t, c.files, c.want)
		})
	}
}

func assertCaseDifferenceRefused(t *testing.T, files []string, want string) {
	t.Helper()
	err := checkExactNames(openRootWith(t, files...), "cfg/p.json")

	var found *ConfigError
	if !errors.As(err, &found) || found.Kind != KindInvalid {
		t.Fatalf("checkExactNames = %v, want an invalid-policy ConfigError", err)
	}
	if !strings.Contains(found.Message, want) || !strings.Contains(found.Message, "project_config_invalid") {
		t.Errorf("message = %q, want it to name %s and carry project_config_invalid", found.Message, want)
	}
}

func TestCheckExactNamesAcceptsTheExactSpelling(t *testing.T) {
	if err := checkExactNames(openRootWith(t, "cfg/p.json"), "cfg/p.json"); err != nil {
		t.Errorf("checkExactNames = %v, want nil", err)
	}
}

func TestCheckExactNamesPrefersTheExactSpellingOverACaseVariant(t *testing.T) {
	root := openRootWith(t, "cfg/p.json", "cfg/P.JSON")
	listing, err := root.Open("cfg")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listing.Close() }()
	names, err := listing.Readdirnames(-1)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Skip("this filesystem folds case, so both spellings cannot exist")
	}

	if err := checkExactNames(root, "cfg/p.json"); err != nil {
		t.Errorf("checkExactNames = %v, want nil when the exact spelling is among the entries", err)
	}
}

func TestCheckExactNamesLeavesAnUnlistedNameToTheOtherChecks(t *testing.T) {
	if err := checkExactNames(openRootWith(t, "cfg/other.json"), "cfg/p.json"); err != nil {
		t.Errorf("checkExactNames = %v, want nil: a name nothing lists is not a case difference", err)
	}
}

func TestFirstCaseVariantReportsTheExactNameBeforeAnyVariantAndTheFirstVariantOtherwise(t *testing.T) {
	cases := []struct {
		name      string
		names     []string
		wantExact bool
		wantName  string
	}{
		{"the exact name alone", []string{"p.json"}, true, "p.json"},
		{"the exact name after a case variant", []string{"P.JSON", "p.json"}, true, "p.json"},
		{"two case variants", []string{"P.json", "P.JSON"}, false, "P.json"},
		{"no entry that matches", []string{"other.json"}, false, "p.json"},
		{"no entries", nil, false, "p.json"},
	}
	for _, c := range cases {
		exact, variant := firstCaseVariant(c.names, "p.json")

		if exact != c.wantExact || variant != c.wantName {
			t.Errorf("%s: firstCaseVariant(%q) = %v, %q; want %v, %q", c.name, c.names, exact, variant, c.wantExact, c.wantName)
		}
	}
}
