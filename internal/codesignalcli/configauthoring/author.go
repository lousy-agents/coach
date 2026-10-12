// Package configauthoring helps a user produce a project-policy candidate:
// guided interactive authoring for TypeScript, and non-interactive root
// suggestion from a committed revision, written create-only.
package configauthoring

import (
	"bufio"
	"io"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

type Result struct {
	// Roots is nil, not a copy of a discovered suggestion, when the user
	// gave no answer -- selection always requires the user's own input.
	Roots []string

	Layers []projectconfig.Layer

	ForbiddenImports []projectconfig.ForbiddenImport

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
	// projectconfig.Load itself uses. It is set only when Approved is true and
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
// stream as the interactive transcript (see Author's doc
// comment): it carries only the document itself, byte for byte. Nothing is
// written when validation fails; a create-only write that finds outputPath
// already occupied leaves the existing content untouched and is reported
// via OutputExists, never as WriteError.
func finalizeApprovedCandidate(result Result, dir string, candidateOut io.Writer, outputPath string, outputSet bool) Result {
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

func collectAuthoringAnswers(in io.Reader, transcript io.Writer, discovered projectmodel.TSRootDiscoveryResult) Result {
	reader := bufio.NewReader(in)
	roots, cancelled := promptForRoots(transcript, reader, discovered)
	if cancelled {
		return Result{Cancelled: true}
	}

	layers, cancelled := promptForLayers(transcript, reader)
	if cancelled {
		return Result{Roots: roots, Layers: layers, Cancelled: true}
	}

	forbidden, cancelled := promptForForbiddenImports(transcript, reader, layers)
	if cancelled {
		return Result{Roots: roots, Layers: layers, ForbiddenImports: forbidden, Cancelled: true}
	}

	requiredLayer, cancelled := promptForRequiredLayer(transcript, reader, layers)
	if cancelled {
		return Result{Roots: roots, Layers: layers, ForbiddenImports: forbidden, Cancelled: true}
	}

	approved := promptForApproval(transcript, reader, discovered, roots, layers, forbidden, requiredLayer)
	return Result{Roots: roots, Layers: layers, ForbiddenImports: forbidden, RequiredLayer: requiredLayer, Approved: approved}
}

// Author runs the guided authoring prompts over in/out rather
// than a real terminal. out is the human-facing transcript; candidateOut
// receives only the approved document when outputSet is false -- mixing
// those streams makes a captured candidate unparseable. Collection never
// preselects roots or infers layers.
func Author(dir string, in io.Reader, out io.Writer, candidateOut io.Writer, discovered projectmodel.TSRootDiscoveryResult, outputPath string, outputSet bool) Result {
	result := collectAuthoringAnswers(in, out, discovered)
	if !result.Approved {
		return result
	}
	return finalizeApprovedCandidate(result, dir, candidateOut, outputPath, outputSet)
}
