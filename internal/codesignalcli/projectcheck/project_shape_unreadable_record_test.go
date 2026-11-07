package projectcheck

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// A record git printed for package.json that Coach cannot read is not an answer: a check that took it for "no
// package.json" would call a project it could not read a project of unknown shape, and exit 0 with the shape
// not checked. The check stops with the operational error instead.
const unreadablePackageRecord = "100644 blob 0123 unexpected-record-without-tab\x00"

// answeringPackageRecord prints record for the tree entry of a package.json, lets the object checks succeed,
// and makes any other question an error that names it.
func answeringPackageRecord(record string) func(string, ...string) ([]byte, error) {
	return func(_ string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "ls-tree" && strings.HasSuffix(args[len(args)-1], "package.json"):
			return []byte(record), nil
		case args[0] == "cat-file":
			return nil, nil
		}
		return nil, fmt.Errorf("answeringPackageRecord: unexpected git question: %s", strings.Join(args, " "))
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
