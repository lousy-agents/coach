package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// foldsCase is whether dir resolves a name spelled in another case to the entry
// that exists, which is what a case-insensitive file system does.
func foldsCase(t *testing.T, dir, respelled string) bool {
	t.Helper()
	_, err := os.Lstat(filepath.Join(dir, respelled))
	return err == nil
}

func configErrorOf(t *testing.T, err error) *ConfigError {
	t.Helper()
	var found *ConfigError
	if !errors.As(err, &found) {
		t.Fatalf("error = %v, want a *ConfigError", err)
	}
	return found
}

// A restore writes where the file system resolves the parents. Where it folds
// case, a policy missing from a parent that exists as CFG would be restored into
// CFG, and the scan then refuses that spelling, so the missing policy is refused
// for its parent's spelling and not offered a restore.
func TestAMissingPolicyUnderAParentSpelledInAnotherCaseIsRefusedWhereTheFileSystemFoldsCase(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "CFG"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !foldsCase(t, dir, "cfg") {
		t.Skip("this file system tells cfg and CFG apart, so a missing cfg/p.json is just missing")
	}

	found := configErrorOf(t, adviseOnRevision(checkRegularFileUnder(dir, "cfg/p.json"), "cfg/p.json", nil))

	if found.Kind != KindInvalid || !strings.Contains(found.Message, `"CFG"`) {
		t.Errorf("error = kind %v, remedy %d, %q; want an invalid policy that names the parent's spelling, with no restore offered", found.Kind, found.Remedy, found.Message)
	}
}

// Where the file system tells the spellings apart, a parent that exists only in
// another case is a different directory, and the policy is simply missing.
func TestAMissingPolicyUnderAParentInAnotherCaseIsJustMissingWhereTheFileSystemTellsThemApart(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "CFG"), 0o755); err != nil {
		t.Fatal(err)
	}
	if foldsCase(t, dir, "cfg") {
		t.Skip("this file system folds case, which the test above covers")
	}

	found := configErrorOf(t, checkRegularFileUnder(dir, "cfg/p.json"))

	if found.Kind != KindMissing {
		t.Errorf("error = kind %v, %q; want a missing policy, since cfg and CFG are different directories here", found.Kind, found.Message)
	}
}

// The policy's own spelling is not a reason to refuse a missing policy: where the
// spellings are different files, P.JSON next to a missing p.json is another file.
func TestAMissingPolicyBesideAFileSpelledInAnotherCaseIsJustMissingWhereTheFileSystemTellsThemApart(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "cfg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cfg", "P.JSON"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if foldsCase(t, filepath.Join(dir, "cfg"), "p.json") {
		t.Skip("this file system folds case, so cfg/p.json exists here")
	}

	found := configErrorOf(t, checkRegularFileUnder(dir, "cfg/p.json"))

	if found.Kind != KindMissing {
		t.Errorf("error = kind %v, %q; want a missing policy: P.JSON is a different file here", found.Kind, found.Message)
	}
}
