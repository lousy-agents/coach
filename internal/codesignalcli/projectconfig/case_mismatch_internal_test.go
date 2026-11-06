package projectconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func fileMismatch() *caseMismatchError {
	return newCaseMismatch("cfg/POLICY.json", []string{"cfg", "POLICY.json"}, 1, "policy.json")
}

func directoryMismatch() *caseMismatchError {
	return newCaseMismatch("CFG/p.json", []string{"CFG", "p.json"}, 0, "cfg")
}

func holdingOnly(held ...string) revisionHolds {
	return func(entry string) (bool, error) { return slices.Contains(held, entry), nil }
}

func TestACaseMismatchAdvisesFromWhatTheRevisionHolds(t *testing.T) {
	cases := []struct {
		name     string
		mismatch *caseMismatchError
		holds    revisionHolds
		want     []string
		avoid    []string
	}{
		{"a file the revision holds as the disk spells it", fileMismatch(), holdingOnly("cfg/policy.json"),
			[]string{`it holds "cfg/policy.json"; pass --project-config cfg/policy.json`}, []string{"rename"}},
		{"a directory the revision holds as the disk spells it", directoryMismatch(), holdingOnly("cfg"),
			[]string{`it holds "cfg"; pass --project-config cfg/p.json`}, []string{"rename"}},
		{"a file the revision holds as the user typed it", fileMismatch(), holdingOnly("cfg/POLICY.json"),
			[]string{`it holds "cfg/POLICY.json"; rename "cfg/policy.json" back to "cfg/POLICY.json", or commit the rename and pass --project-config cfg/policy.json`}, nil},
		{"a directory the revision holds as the user typed it", directoryMismatch(), holdingOnly("CFG"),
			[]string{`it holds "CFG"; rename "cfg" back to "CFG", or commit the rename and pass --project-config cfg/p.json`}, nil},
		{"a file the revision holds in neither spelling", fileMismatch(), holdingOnly(),
			[]string{`it holds neither "cfg/POLICY.json" nor "cfg/policy.json"`}, []string{"rename", "pass --project-config"}},
		{"a directory the revision holds in neither spelling", directoryMismatch(), holdingOnly("somewhere/else"),
			[]string{`it holds neither "CFG" nor "cfg"`}, []string{"rename", "pass --project-config"}},
		{"a file the revision holds in both spellings", fileMismatch(), holdingOnly("cfg/POLICY.json", "cfg/policy.json"),
			[]string{`it holds both "cfg/POLICY.json" and "cfg/policy.json"`}, []string{"rename", "pass --project-config"}},
		{"a revision that cannot be asked", fileMismatch(), func(string) (bool, error) { return false, errors.New("boom") },
			[]string{`is stored as "policy.json" on disk, not "POLICY.json"`}, []string{"it holds", "rename", "pass --project-config"}},
		{"no way to ask the revision", fileMismatch(), nil,
			[]string{`is stored as "policy.json" on disk, not "POLICY.json"`}, []string{"it holds", "rename", "pass --project-config"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertCaseMismatchAdvice(t, c.mismatch.config(c.holds), c.want, c.avoid)
		})
	}
}

func assertCaseMismatchAdvice(t *testing.T, got *ConfigError, want, avoid []string) {
	t.Helper()
	if got.Kind != KindInvalid || !strings.Contains(got.Message, "(project_config_invalid)") {
		t.Errorf("got %v %q; want an invalid-policy refusal that carries project_config_invalid", got.Kind, got.Message)
	}
	for _, w := range want {
		if !strings.Contains(got.Message, w) {
			t.Errorf("message %q; want it to contain %q", got.Message, w)
		}
	}
	for _, a := range avoid {
		if strings.Contains(got.Message, a) {
			t.Errorf("message %q; want it not to contain %q", got.Message, a)
		}
	}
}

func TestAMismatchThatReachesAnErrorWithoutAdviceStillRefusesAndNamesBothSpellings(t *testing.T) {
	got := fileMismatch().Error()

	if !strings.Contains(got, `is stored as "policy.json" on disk, not "POLICY.json"`) || !strings.Contains(got, "(project_config_invalid)") {
		t.Errorf("Error() = %q; want the refusal that names both spellings", got)
	}
}
