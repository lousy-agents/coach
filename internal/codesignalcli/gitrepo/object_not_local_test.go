package gitrepo

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func exitedWith(t *testing.T, code string) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit "+code).Run()
	if err == nil {
		t.Fatalf("exit %s did not fail", code)
	}
	return err
}

func TestAnObjectCheckThatExitsOneIsAnObjectThatIsNotLocal(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"git's silent exit 1", exitedWith(t, "1"), true},
		{"a fatal error", exitedWith(t, "128"), false},
		{"an error that is not an exit", errors.New("boom"), false},
		{"git's exit 1 with a message after it, which is not the silent form", errors.New("exit status 1: fatal: bad object"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertObjectNotLocal(t, c.err, c.want)
		})
	}
}

func assertObjectNotLocal(t *testing.T, err error, want bool) {
	t.Helper()
	var absent *ObjectNotLocalError
	classified := classifyObjectCheck("abc123", err)
	if got := errors.As(classified, &absent); got != want {
		t.Errorf("classifyObjectCheck(%v) is an ObjectNotLocalError = %v; want %v", err, got, want)
	}
	if !want && !errors.Is(classified, err) {
		t.Errorf("classifyObjectCheck(%v) = %v; want git's own error back, so that its cause is kept", err, classified)
	}
}

func TestAPolicyBlobThatAPartialCloneLacksIsReportedAsNotLocal(t *testing.T) {
	skipIfGitCannotRefuseLazyFetch(t)
	clone, _ := partialClone(t, func(string) string { return "ssh://example.invalid/repo.git" })

	_, _, err := BlobEntryAtRevision(BoundedRunner(1<<20, 1<<16, 30*time.Second), clone, "HEAD", "cfg/p.json")

	var operational *OperationalError
	var absent *ObjectNotLocalError
	if !errors.As(err, &operational) || !errors.As(operational.Cause, &absent) {
		t.Fatalf("BlobEntryAtRevision error = %#v; want an OperationalError caused by an ObjectNotLocalError", err)
	}
	if strings.Contains(operational.Message, "exit status") || !strings.Contains(operational.Message, "is not in the local object store") {
		t.Errorf("message %q; want the object named as missing, not git's bare exit status", operational.Message)
	}
}
