package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestEveryLoadErrorStatesWhetherANewPolicyWouldHelp(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, repo string)
		kind  ErrorKind
		want  projectreadiness.PolicyRemedy
	}{
		{"a file on disk that is not committed is committed, not authored", func(t *testing.T, repo string) { writeFile(t, repo, "pol.json", "{}") }, KindUncommitted, projectreadiness.RemedyNone},
		{"a path found nowhere is authored", func(t *testing.T, repo string) {}, KindNotFound, projectreadiness.RemedyAuthor},
		{"a committed policy that fails validation is authored again", func(t *testing.T, repo string) { gitfixture.CommitFile(t, repo, "pol.json", "not json") }, KindInvalid, projectreadiness.RemedyAuthor},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertLoadErrorRemedy(t, c.setup, c.kind, c.want)
		})
	}
}

func assertLoadErrorRemedy(t *testing.T, setup func(t *testing.T, repo string), kind ErrorKind, want projectreadiness.PolicyRemedy) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.go", "package a\n")
	setup(t, repo)

	_, err := Load(repo, "HEAD", "pol.json")

	var configErr *ConfigError
	if !errors.As(err, &configErr) || configErr.Kind != kind || configErr.Remedy != want {
		t.Errorf("Load error = %+v; want kind %d with remedy %d", configErr, kind, want)
	}
}

func TestAPolicyOverTheSizeBudgetKeepsTheBoundTextAndIsAuthoredAgain(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.go", "package a\n")
	gitfixture.CommitFile(t, repo, "pol.json", strings.Repeat(" ", MaxBytes+1)+"{}")

	_, err := Load(repo, "HEAD", "pol.json")

	var configErr *ConfigError
	if !errors.As(err, &configErr) || configErr.Kind != KindUnreadable {
		t.Fatalf("Load error = %v; want a KindUnreadable *ConfigError for a policy larger than the read budget", err)
	}
	if configErr.Remedy != projectreadiness.RemedyAuthor {
		t.Errorf("Remedy = %d; want RemedyAuthor, because a smaller policy fixes a policy over the size budget", configErr.Remedy)
	}
	for _, want := range []string{"is not readable at revision", "git stdout exceeded"} {
		if !strings.Contains(configErr.Message, want) {
			t.Errorf("message %q lacks %q; the text of the bound that tripped must stay", configErr.Message, want)
		}
	}
}

func TestAReadThatTripsTheTimeOrStderrBoundOffersNoNewPolicy(t *testing.T) {
	for name, kind := range map[string]gitrepo.BoundErrorKind{"time": gitrepo.BoundTimeout, "stderr": gitrepo.BoundStderr} {
		t.Run(name, func(t *testing.T) {
			assertBoundReadOffersNoNewPolicy(t, kind)
		})
	}
}

func assertBoundReadOffersNoNewPolicy(t *testing.T, kind gitrepo.BoundErrorKind) {
	t.Helper()

	err := readError("", "HEAD", "pol.json", &gitrepo.BoundError{Kind: kind})

	var configErr *ConfigError
	if !errors.As(err, &configErr) || configErr.Kind != KindUnreadable {
		t.Fatalf("readError = %v; want a KindUnreadable *ConfigError", err)
	}
	if configErr.Remedy != projectreadiness.RemedyNone {
		t.Errorf("Remedy = %d; want RemedyNone, because a new policy does not change why the read failed", configErr.Remedy)
	}
	if !strings.Contains(configErr.Message, "is not readable at revision") {
		t.Errorf("message %q; want the not-readable message, which carries the bound's own text", configErr.Message)
	}
}
