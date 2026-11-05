package codesignalcli

import "testing"

func TestProjectBackendRequestNamesOnlyAPolicyTheUserPassed(t *testing.T) {
	cases := []struct {
		name     string
		explicit bool
		want     string
	}{
		{"a policy named with --project-config", true, ".lousy-agents/coach.json"},
		{"a policy the scan discovered", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertExplicitConfigPath(t, c.explicit, c.want)
		})
	}
}

func assertExplicitConfigPath(t *testing.T, explicit bool, want string) {
	t.Helper()
	request := ProjectBackendRequest{ConfigPath: ".lousy-agents/coach.json", ConfigExplicit: explicit}

	if got := request.ExplicitConfigPath(); got != want {
		t.Errorf("ExplicitConfigPath() = %q, want %q", got, want)
	}
}
