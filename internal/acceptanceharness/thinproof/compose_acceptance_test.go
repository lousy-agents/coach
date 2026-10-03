//go:build thinproof

package thinproof_test

import (
	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

// composeDir is deploy/compose/thinproof relative to this test file's
// package directory (internal/acceptanceharness/thinproof).
const composeDir = "../../../deploy/compose/thinproof"

// thinproofResult mirrors cmd/thinproof-runner's on-disk result.json shape.
// It is redefined here (rather than imported) because cmd/thinproof-runner
// is package main and not importable; every field type is the same public
// type the runner itself uses, so this is not a competing schema.
type thinproofResult struct {
	SchemaVersion     int                                     `json:"schema_version"`
	Report            *codesignal.Report                      `json:"report"`
	FileMetadata      githubingest.FileMetadata               `json:"file_metadata"`
	GuardResult       acceptanceharness.CredentialGuardResult `json:"guard_result"`
	BlockedRequests   []string                                `json:"blocked_requests"`
	FakeGitHubRecords []acceptanceharness.RequestRecord       `json:"fake_github_records"`
}
