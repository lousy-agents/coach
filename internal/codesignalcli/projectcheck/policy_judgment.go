package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// policyJudgment is what the check concluded about the policy: the path it judged, the check itself, and, for a policy
// that passed, the roots it gave the other checks and the warnings that say how it relates to the analyzed revision.
type policyJudgment struct {
	path     string
	check    projectreadiness.Check
	roots    []string
	warnings []projectreadiness.Warning
}

// judgePolicy judges the policy from the working tree: a usable regular file there is the policy, whatever the
// analyzed revision and the index hold for its path (ADR-009). Only a path with no usable file on disk asks the revision
// and the index which way out there is. A root project.json that discovery falls back to is a policy only when it is a
// valid one: an invalid or unreadable one is no policy, and the gap is the one for a policy that is missing.
func judgePolicy(run gitrepo.Runner, dir, revision string, policy policyref.Ref) (policyJudgment, error) {
	if _, err := gitrepo.RepositoryRootWith(run, dir); err != nil {
		return policyJudgment{}, err
	}
	namedPath, _ := policy.NamedPath()
	discovered := namedPath == ""
	selection, err := projectconfig.SelectOnDisk(dir, namedPath)
	if err != nil {
		failedAt := selectionFailurePath(dir, namedPath)
		if discovered && failedAt == projectconfig.DefaultPath {
			return judgeAbsentPolicy(run, dir, revision, policy)
		}
		return judgmentOfRefusal(failedAt, err)
	}
	if selection.Path == "" {
		return judgeAbsentPolicy(run, dir, revision, policy)
	}
	config, err := projectconfig.UsableDiskPolicy(selection)
	if err != nil {
		if discovered && selection.Path == projectconfig.DefaultPath {
			return judgeAbsentPolicy(run, dir, revision, policy)
		}
		return judgmentOfRefusal(selection.Path, err)
	}
	return judgeSelectedPolicy(run, dir, revision, selection, config, policy)
}

// selectionFailurePath names the file a failed selection stopped at. SelectOnDisk reports the error and not the path,
// and it stops at the first candidate that is a regular file it cannot read, so asking for the canonical file alone
// tells whether that was the one.
func selectionFailurePath(dir, namedPath string) string {
	if namedPath != "" {
		return namedPath
	}
	if _, err := projectconfig.SelectOnDisk(dir, projectconfig.CanonicalPath); err != nil {
		return projectconfig.CanonicalPath
	}
	return projectconfig.DefaultPath
}

// judgmentOfRefusal is the policy_invalid gap for a regular file that cannot be used. Authoring refuses a path that
// exists, so the gap offers no way out: a file over the size limit or with content that is no policy carries the cause
// undetermined, and one the process could not read carries unreadable_on_disk.
func judgmentOfRefusal(path string, cause error) (policyJudgment, error) {
	refusal := projectconfig.ClassifyDiskPolicyError(path, cause).ExplicitError()
	check, _, err := policyGapFrom(refusal)
	return policyJudgment{path: path, check: check}, err
}

// judgeSelectedPolicy passes a usable policy on disk with the roots of the bytes read, and says how it relates to the
// analyzed revision. Two states outrank the file: it lies inside a submodule checkout, where the superproject can
// never commit it, or the revision holds the policy only in another case, which the revision still decides.
func judgeSelectedPolicy(run gitrepo.Runner, dir, revision string, selection projectconfig.DiskSelection, config projectconfig.Config, policy policyref.Ref) (policyJudgment, error) {
	evidence := readHeadEvidence(run, dir, revision, selection)
	if evidence.submoduleMessage != "" {
		check := projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing}
		return policyJudgment{path: selection.Path, check: check.WithCause(evidence.submoduleMessage, projectreadiness.CauseInSubmodule)}, nil
	}
	if evidence.copy == headHoldsNoFile {
		if renamed := projectconfig.WrongCaseAtRevision(run, dir, revision, selection.Path); renamed != nil {
			if projectconfig.GitSilence(renamed, "coach codesignal --check-project", selection.Path) != nil {
				evidence.unanswered = true
			} else {
				return judgmentOfWrongCase(renamed, selection.Path, policy)
			}
		}
	}
	judgment := policyJudgment{path: selection.Path, check: projectreadiness.Check{State: projectreadiness.Pass}, roots: config.Roots}
	if !policy.Explicit && selection.Path == projectconfig.DefaultPath {
		judgment.warnings = append(judgment.warnings, projectreadiness.Warning{Code: projectreadiness.WarnProjectConfigDeprecated})
	}
	if code := evidence.warningCode(); code != "" {
		judgment.warnings = append(judgment.warnings, projectreadiness.Warning{Code: code, Root: selection.Path})
	}
	return judgment, nil
}

func judgmentOfWrongCase(renamed error, path string, policy policyref.Ref) (policyJudgment, error) {
	check, _, err := policyGapFrom(renamed)
	if err != nil {
		return policyJudgment{}, err
	}
	return policyJudgment{path: path, check: wordedForThePolicy(check, policy, path)}, nil
}

// wordedForThePolicy is check worded as the scan words it: a policy Coach found is the default policy, and not a
// --project-config the user never passed.
func wordedForThePolicy(check projectreadiness.Check, policy policyref.Ref, path string) projectreadiness.Check {
	if !policy.JudgedOnDisk() {
		check.CauseText = projectconfig.WordedAsTheDefaultPolicy(check.CauseText, check.PolicyCause, path)
	}
	return check
}
