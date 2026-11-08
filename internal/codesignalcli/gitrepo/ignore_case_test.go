package gitrepo

import (
	"errors"
	"testing"
)

// core.ignorecase is asked as a boolean. An unset key is git's plain no; a failure to answer is not a no,
// because a repository whose answer is unknown must not be taken for one that tells cases apart.
func TestIgnoresCaseReadsWhatGitSaysAndRefusesToGuessWhenItSaysNothing(t *testing.T) {
	cases := []struct {
		name    string
		output  string
		err     error
		want    bool
		wantErr bool
	}{
		{"core.ignorecase is true", "true\n", nil, true, false},
		{"core.ignorecase is false", "false\n", nil, false, false},
		{"core.ignorecase is unset", "", exitStatusOne, false, false},
		{"git prints nothing", "", nil, false, false},
		{"a damaged configuration", "", exitStatus128, false, true},
		{"a read past the time bound", "", &BoundError{Kind: BoundTimeout, message: "git execution timed out"}, false, true},
		{"git cannot be run", "", errors.New("exec: git: not found"), false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertIgnoresCase(t, c.output, c.err, c.want, c.wantErr)
		})
	}
}

func assertIgnoresCase(t *testing.T, output string, err error, want, wantErr bool) {
	t.Helper()
	run := func(string, ...string) ([]byte, error) { return []byte(output), err }

	got, gotErr := IgnoresCase(run, "repo")

	if got != want || (gotErr != nil) != wantErr {
		t.Errorf("IgnoresCase = %v, %v; want %v with error = %v", got, gotErr, want, wantErr)
	}
}

func TestIgnoresCaseAsksGitToReadTheSettingAsABoolean(t *testing.T) {
	var asked []string
	run := func(_ string, args ...string) ([]byte, error) {
		asked = args
		return []byte("true\n"), nil
	}

	_, _ = IgnoresCase(run, "repo")

	want := []string{"config", "--type=bool", "--get", "core.ignorecase"}
	if len(asked) != len(want) {
		t.Fatalf("asked %q; want %q", asked, want)
	}
	for i := range want {
		if asked[i] != want[i] {
			t.Errorf("asked %q; want %q", asked, want)
		}
	}
}
