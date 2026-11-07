package gitrepo

import (
	"errors"
	"testing"
)

// git config says "nothing matched" with exit 1 and nothing on stderr. Any other exit, any text around the
// exit, a bound and a missing error are not that answer.
func TestConfigHasNoValueIsGitsExitOneAndNothingElse(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"exit 1":                  {exitStatusOne, true},
		"a fatal exit":            {exitStatus128, false},
		"exit 1 with git's words": {errors.New("exit status 1: error: key does not contain a section"), false},
		"a bound":                 {&BoundError{Kind: BoundTimeout, message: "timed out"}, false},
		"no error":                {nil, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireNoValue(t, c.err, c.want) })
	}
}

func requireNoValue(t *testing.T, err error, want bool) {
	t.Helper()
	if got := ConfigHasNoValue(err); got != want {
		t.Errorf("ConfigHasNoValue(%v) = %v, want %v", err, got, want)
	}
}
