package githubingest_test

import (
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// AC-1.1 regression guard: pkg/semantics shall not import the GitHub App
// dependencies introduced by this package (go-github, ghinstallation).
func TestPackageBoundary_SemanticsDoesNotImportGithubDeps(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/lousy-agents/coach/pkg/semantics/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps github.com/lousy-agents/coach/pkg/semantics/... failed: %v\noutput:\n%s", err, out)
	}
	if strings.Contains(string(out), "go-github") {
		t.Fatalf("pkg/semantics must not depend on go-github, but dependency list contained it:\n%s", out)
	}
	if strings.Contains(string(out), "ghinstallation") {
		t.Fatalf("pkg/semantics must not depend on ghinstallation, but dependency list contained it:\n%s", out)
	}
}

// AC-5.2: constructor validation - an incomplete GitHubAppConfig (missing
// AppID, InstallationID, or PrivateKey) must be rejected before any client
// is built.
func TestNewGitHubFileReader_RejectsIncompleteConfig(t *testing.T) {
	validKey := generateTestRSAPrivateKeyPEM(t)

	tests := map[string]githubingest.GitHubAppConfig{
		"missing AppID": {
			InstallationID: 67890,
			PrivateKey:     validKey,
		},
		"missing InstallationID": {
			AppID:      12345,
			PrivateKey: validKey,
		},
		"missing PrivateKey": {
			AppID:          12345,
			InstallationID: 67890,
		},

		"negative AppID": {
			AppID:          -1,
			InstallationID: 67890,
			PrivateKey:     validKey,
		},
		"negative InstallationID": {
			AppID:          12345,
			InstallationID: -1,
			PrivateKey:     validKey,
		},
	}

	for name, cfg := range tests {
		t.Run(name, func(t *testing.T) {
			body_readerPart2Test_63(t, cfg)
		})
	}
}

// AC-5.6: 401 and 403 responses from the Contents API both surface as
// ErrAuth.
func TestReadFile_UnauthorizedOrForbiddenStatusReturnsErrAuth(t *testing.T) {
	tests := map[string]int{
		"401 Unauthorized": http.StatusUnauthorized,
		"403 Forbidden":    http.StatusForbidden,
	}

	for name, status := range tests {
		t.Run(name, func(t *testing.T) {
			body_readerPart2Test_84(t, status)
		})
	}
}

// AC-5.1: pkg/githubingest shall not import pkg/semantics.
func TestPackageBoundary_GithubingestDoesNotImportSemantics(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/lousy-agents/coach/pkg/githubingest/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps github.com/lousy-agents/coach/pkg/githubingest/... failed: %v\noutput:\n%s", err, out)
	}
	if strings.Contains(string(out), "lousy-agents/coach/pkg/semantics") {
		t.Fatalf("pkg/githubingest must not depend on pkg/semantics, but dependency list contained it:\n%s", out)
	}
}
