package evalhistory

import (
	"errors"
	"regexp"
)

var errNotImplemented = errors.New("evalhistory: not implemented")

var FixPattern = regexp.MustCompile(`^$`)

type FixCommit struct{ SHA, Subject string }

type FixScan struct {
	Commits int
	Fixes   []FixCommit
}

func FindFixes(repoDir, since, until string) (FixScan, error) { return FixScan{}, errNotImplemented }

type Unit struct {
	Path       string
	Start, End int
}

type Involvement struct {
	FixSHA   []string
	Unmapped int
}

func Involve(repoDir, since string, units []Unit, fixes []FixCommit) (Involvement, error) {
	return Involvement{}, errNotImplemented
}
