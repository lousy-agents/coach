package projectcheck

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// A record git printed for package.json that Coach cannot read is not an answer: a check that took it for "no
// package.json" would call a project it could not read a project of unknown shape, and exit 0 with the shape
// not checked. The check stops with the operational error instead.
const unreadablePackageRecord = "100644 blob 0123 unexpected-record-without-tab\x00"

func answeringPackageRecord(record string) func(string, ...string) ([]byte, error) {
	return func(_ string, args ...string) ([]byte, error) {
		if args[0] == "ls-tree" && strings.HasSuffix(args[len(args)-1], "package.json") {
			return []byte(record), nil
		}
		return nil, nil
	}
}

func TestAPackageManifestRecordCoachCannotReadStopsTheShapeCheck(t *testing.T) {
	withReadinessGit(t, answeringPackageRecord(unreadablePackageRecord))

	check, err := checkProjectShape("repo", "HEAD", nil, false)

	requireOperationalError(t, check, err)
}

func TestAPackageManifestRecordCoachCannotReadStopsTheWalkUpFromARoot(t *testing.T) {
	withReadinessGit(t, answeringPackageRecord(unreadablePackageRecord))

	found, err := packageJSONExistsUnderAnyRoot("repo", "HEAD", []string{"apps/web"})

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) || found {
		t.Errorf("packageJSONExistsUnderAnyRoot = %v, %v; want no answer and the operational error", found, err)
	}
}
