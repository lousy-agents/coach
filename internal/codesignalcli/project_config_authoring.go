package codesignalcli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

type AuthoringResult struct {
	// Roots is nil, not a copy of a discovered suggestion, when the user
	// gave no answer -- selection always requires the user's own input.
	Roots []string

	Layers []projectConfigLayer

	ForbiddenImports []projectForbiddenImport

	RequiredLayer string

	// Approved reports whether the user gave the exact approval token at
	// the candidate/coverage-preview gate. It is only ever true when every
	// earlier stage completed without cancellation and the user then typed
	// the approval token itself; nothing is written on the strength of this
	// field alone -- that is a later stage's job.
	//
	// Declining approval (any answer other than the approval token,
	// including a blank answer or an exhausted/erroring read) is NOT a
	// cancellation: Cancelled stays false in that case. A caller that
	// writes a config MUST gate that write on Approved being true --
	// gating on !Cancelled alone is wrong and would write a config the
	// user explicitly declined to approve.
	Approved bool

	// Cancelled reports whether the user cancelled the session instead of
	// resolving an invalid answer. Roots/Layers/ForbiddenImports/RequiredLayer
	// hold whatever had already been accepted up to that point; none of
	// them are a complete, validated candidate when this is true.
	//
	// Exhausted input is ambiguous with a deliberate blank answer at a
	// stage-terminating prompt (roots, "finish defining layers", "finish
	// forbidden pairs", "leave blank for none"): both end that stage the
	// same way, so Cancelled stays false. Only exhausted input at a
	// retry-or-cancel prompt (an invalid answer with no more input to
	// correct it) sets Cancelled to true.
	//
	// Declining the final approval gate also leaves Cancelled false --
	// that is a distinct outcome (Approved == false), not a cancellation.
	// A caller that writes a config on the strength of !Cancelled alone,
	// without also checking Approved, will write a config the user
	// explicitly declined.
	Cancelled bool

	// Document holds the approved candidate rendered as the schema-1
	// project-config document, once it has passed the same schema validator
	// LoadProjectConfig itself uses. It is set only when Approved is true and
	// ValidationError is nil; it is unaffected by whether writing that
	// document to disk was itself refused or failed (see OutputExists,
	// WriteError).
	Document []byte

	// ValidationError holds a schema-validation failure of the approved
	// candidate. Every field the interactive flow itself can produce --
	// including root selection, which is validated at collection time and
	// can never reach the approval gate empty or malformed -- is already
	// checked as it is collected, so in practice this is defense in depth
	// against a scenario the interactive flow cannot reach on its own (e.g.
	// buildApprovedCandidate called directly with a shape
	// validateForbiddenPairCandidate already rejects during collection), not
	// something a real guided-authoring session can trigger. Its being set
	// means nothing was written and Document is nil.
	ValidationError error

	// OutputExists reports whether the caller-selected output path already
	// existed, so the create-only write was refused without touching its
	// existing content. Only meaningful when Approved is true, outputSet was
	// true, and ValidationError is nil.
	OutputExists bool

	// WriteError holds a failure writing the approved candidate: when
	// outputSet was true, a create-only write failure other than the target
	// already existing (an invalid or unconfined output path, or an
	// unexpected filesystem error); when outputSet was false, a failure
	// writing the candidate to the caller-supplied candidateOut (e.g. a
	// broken pipe or full disk on the other end). Only meaningful when
	// Approved is true and ValidationError is nil.
	WriteError error
}

// finalizeApprovedCandidate validates result's collected fields and either
// create-only writes them to outputPath (outputSet true) or emits them to
// candidateOut (outputSet false). candidateOut must never be the same
// stream as the interactive transcript (see AuthorProjectConfig's doc
// comment): it carries only the document itself, byte for byte. Nothing is
// written when validation fails; a create-only write that finds outputPath
// already occupied leaves the existing content untouched and is reported
// via OutputExists, never as WriteError.
func finalizeApprovedCandidate(result AuthoringResult, dir string, candidateOut io.Writer, outputPath string, outputSet bool) AuthoringResult {
	candidate, err := buildApprovedCandidate(result.Roots, result.Layers, result.ForbiddenImports, result.RequiredLayer)
	if err != nil {
		result.ValidationError = err
		return result
	}
	result.Document = candidate

	if !outputSet {
		if _, err := candidateOut.Write(candidate); err != nil {
			result.WriteError = err
		}
		return result
	}

	clean, pathErr := validateOutputPath(dir, outputPath)
	if pathErr != nil {
		result.WriteError = pathErr
		return result
	}
	exists, writeErr := writeSuggestOutput(dir, clean, candidate)
	if exists {
		result.OutputExists = true
		return result
	}
	if writeErr != nil {
		result.WriteError = writeErr
		return result
	}
	return result
}

// promptForApproval prints the complete candidate together with its coverage
// preview -- each named layer's prefixes paired with the discovered
// directories they match, and every discovered directory no declared layer
// matches -- and then reads one answer. Only the exact approval token
// (case-insensitive) approves; anything else, including a blank answer or an
// exhausted/erroring read, does not. There is no retry here: the point of
// this gate is that the user either approves what was just shown or they
// don't, so a single read is always enough to decide it.
func promptForApproval(out io.Writer, reader *bufio.Reader, discovered projectmodel.TSRootDiscoveryResult, roots []string, layers []projectConfigLayer, forbidden []projectForbiddenImport, requiredLayer string) bool {
	printCandidateSummary(out, roots, layers, forbidden, requiredLayer)
	printCoveragePreview(out, discovered, layers)

	fmt.Fprintln(out, "Type 'approve' to write this project config, or anything else to cancel without writing:")
	fmt.Fprint(out, "> ")
	answer, _ := prompt.ReadLine(reader)
	return strings.EqualFold(strings.TrimSpace(answer), "approve")
}

// directoryHasPrefix reports whether prefix matches dir the same way a real
// layer-violation evaluation would (pkg/codesignal/rule_layer_violation_match.go's
// layerContainsDir): dir equals prefix, dir is nested under prefix, or prefix
// is ".", the universal repository-root ancestor.
func directoryHasPrefix(dir, prefix string) bool {
	return prefix == "." || dir == prefix || strings.HasPrefix(dir, prefix+"/")
}

func printRootSuggestions(out io.Writer, discovered projectmodel.TSRootDiscoveryResult) {
	if len(discovered.Roots) > 0 {
		fmt.Fprintln(out, "Discovered TypeScript roots (directories with a tsconfig.json):")
		for i, root := range discovered.Roots {
			fmt.Fprintf(out, "  %d. %s\n", i+1, root)
		}
	} else {
		fmt.Fprintln(out, "No TypeScript roots (tsconfig.json) were discovered.")
	}
	if len(discovered.Candidates) > 0 {
		fmt.Fprintln(out, "Other directories with a package.json but no tsconfig.json of their own:")
		for _, candidate := range discovered.Candidates {
			fmt.Fprintf(out, "  - %s\n", candidate)
		}
	}
}
