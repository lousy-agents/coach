package projectconfig

import (
	"errors"
	"strings"
	"testing"
)

func fileMismatch() caseMismatchError {
	return newCaseMismatch("cfg/POLICY.json", []string{"cfg", "POLICY.json"}, 1, "policy.json")
}

func directoryMismatch() caseMismatchError {
	return newCaseMismatch("CFG/p.json", []string{"CFG", "p.json"}, 0, "cfg")
}

// fakeRevision is a revision's tree as the names directly under each directory, "" being the root.
type fakeRevision map[string][]string

func (f fakeRevision) entries(directory string) ([]string, error) {
	return f[directory], nil
}

func failingRevision(string) ([]string, error) {
	return nil, errors.New("boom")
}

func TestACaseMismatchAdvisesFromWhatTheRevisionHolds(t *testing.T) {
	cases := []struct {
		name     string
		mismatch caseMismatchError
		entries  revisionEntries
		want     []string
		avoid    []string
	}{
		{"a file the revision holds as the disk spells it", fileMismatch(), fakeRevision{"": {"cfg"}, "cfg": {"policy.json"}}.entries,
			[]string{`it holds "cfg/policy.json"; pass --project-config cfg/policy.json`}, []string{"rename"}},
		{"a directory the revision holds as the disk spells it", directoryMismatch(), fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries,
			[]string{`it holds "cfg"; pass --project-config cfg/p.json`}, []string{"rename"}},
		{"a file the revision holds as the user typed it", fileMismatch(), fakeRevision{"": {"cfg"}, "cfg": {"POLICY.json"}}.entries,
			[]string{`it holds "cfg/POLICY.json"; rename "cfg/policy.json" back to "cfg/POLICY.json", or commit the rename and pass --project-config cfg/policy.json`}, nil},
		{"a directory the revision holds as the user typed it", directoryMismatch(), fakeRevision{"": {"CFG"}, "CFG": {"p.json"}}.entries,
			[]string{`it holds "CFG"; rename "cfg" back to "CFG", or commit the rename and pass --project-config cfg/p.json`}, nil},
		{"a file the revision holds in neither spelling", fileMismatch(), fakeRevision{"": {"cfg"}, "cfg": {"other.json"}}.entries,
			[]string{`it holds neither "cfg/POLICY.json" nor "cfg/policy.json"`}, []string{"rename", "pass --project-config"}},
		{"a directory the revision holds in neither spelling", directoryMismatch(), fakeRevision{"": {"somewhere"}}.entries,
			[]string{`it holds neither "CFG" nor "cfg"`}, []string{"rename", "pass --project-config"}},
		{"a file the revision holds in both spellings", fileMismatch(), fakeRevision{"": {"cfg"}, "cfg": {"POLICY.json", "policy.json"}}.entries,
			[]string{`it holds both "cfg/POLICY.json" and "cfg/policy.json"`}, []string{"rename", "pass --project-config"}},
		{"a revision that cannot be asked", fileMismatch(), failingRevision,
			[]string{`is stored as "policy.json" on disk, not "POLICY.json"`}, []string{"it holds", "rename", "pass --project-config"}},
		{"no way to ask the revision", fileMismatch(), nil,
			[]string{`is stored as "policy.json" on disk, not "POLICY.json"`}, []string{"it holds", "rename", "pass --project-config"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertCaseMismatchAdvice(t, c.mismatch.config(c.entries), c.want, c.avoid)
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
