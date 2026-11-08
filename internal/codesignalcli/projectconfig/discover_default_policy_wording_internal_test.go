package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// A scan that refuses a policy it found by itself must not say the user passed it: the message names the
// default policy, and the flag --project-config appears only where the user used it.
func TestAMessageAboutADiscoveredPolicyNamesItAsTheDefaultPolicyNotAsAFlag(t *testing.T) {
	rows := map[string]func(t *testing.T, repo string){
		"content that is not valid": func(t *testing.T, repo string) { gitfixture.CommitFile(t, repo, CanonicalPath, "{") },
		"a link":                    func(t *testing.T, repo string) { gitfixture.CommitLink(t, repo, CanonicalPath, "real.json") },
		"a directory":               func(t *testing.T, repo string) { gitfixture.CommitFile(t, repo, CanonicalPath+"/inner.json", "{}") },
	}
	for name, commit := range rows {
		t.Run(name, func(t *testing.T) { requireTheDefaultPolicyWording(t, commit) })
	}
}

func requireTheDefaultPolicyWording(t *testing.T, commit func(t *testing.T, repo string)) {
	t.Helper()
	repo := committedRepository(t)
	commit(t, repo)

	selection, err := Discover(repo, "HEAD")

	var refusal *ConfigError
	if err != nil || !errors.As(selection.Fatal, &refusal) {
		t.Fatalf("Discover = %+v, %v; want the policy refused", selection, err)
	}
	if !strings.Contains(refusal.Message, `the default policy ".lousy-agents/coach.json"`) || strings.Contains(refusal.Message, "--project-config") {
		t.Errorf("message %q; want the default policy named and no flag", refusal.Message)
	}
}

// The same error for a policy the user named still says so: the flag is theirs.
func TestAMessageAboutANamedPolicyStillNamesTheFlag(t *testing.T) {
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{")

	_, err := Load(repo, "HEAD", "cfg/p.json")

	var refusal *ConfigError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, `--project-config "cfg/p.json"`) {
		t.Errorf("Load error = %v; want the user's flag named", err)
	}
}

// The legacy default policy is found by the scan as well. When it cannot be read (here, over the size budget)
// the error that stops discovery names it as the default policy too.
func TestAnErrorAboutTheLegacyDefaultPolicyNamesItAsTheDefaultPolicyToo(t *testing.T) {
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, DefaultPath, strings.Repeat("x", MaxBytes+1))

	_, err := Discover(repo, "HEAD")

	var refusal *ConfigError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, `the default policy "`+DefaultPath+`"`) || strings.Contains(refusal.Message, "--project-config") {
		t.Errorf("Discover error = %v; want the legacy default policy named and no flag", err)
	}
}
