package codesignalcli

import (
	"bufio"

	"fmt"
	"io"

	"strings"

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

// AuthorProjectConfig runs the guided authoring prompts over in/out rather
// than a real terminal. out is the human-facing transcript; candidateOut
// receives only the approved document when outputSet is false -- mixing
// those streams makes a captured candidate unparseable. Collection never
// preselects roots or infers layers.

// buildApprovedCandidate renders the collected fields as the schema-1
// project-config document and runs it through parseProjectConfig -- the same
// validator LoadProjectConfig applies to a committed --project-config file
// -- before treating it as valid. source_sink_pack is never populated: it is
// a reserved field this feature must not touch.

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
	answer, _ := readLine(reader)
	return strings.EqualFold(strings.TrimSpace(answer), "approve")
}

// discoveredDirectories returns discovered.Roots and discovered.Candidates
// combined into one ordered, deduplicated list -- the coverage preview
// treats every directory DiscoverTSRoots found as something the eventual
// policy should account for, not only the ones it called out as tsconfig
// roots.

// directoryHasPrefix reports whether prefix matches dir the same way a real
// layer-violation evaluation would (pkg/codesignal/rule_layer_violation_match.go's
// layerContainsDir): dir equals prefix, dir is nested under prefix, or prefix
// is ".", the universal repository-root ancestor.
func directoryHasPrefix(dir, prefix string) bool {
	return prefix == "." || dir == prefix || strings.HasPrefix(dir, prefix+"/")
}

// promptForRoots prints discovered.Roots/Candidates as a suggestion, then
// repeatedly reads one line of the user's own answer until it resolves to a
// non-empty set of valid repository-relative directories -- the same
// explain-and-retry-or-cancel treatment every other field (layers, forbidden
// pairs, required layer) already gets. There is no answer that selects zero
// roots: validateProjectConfigRoots (the same schema validator the write
// stage applies) never accepts an empty root list, so accepting one here
// would only defer a certain failure to the very end of the session, after
// every remaining stage and the approval gate had already been answered. It
// never returns discovered.Roots itself when the user selects nothing: an
// accepted selection is always something the user typed or picked.

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

// readLine sets unreadable on any read error, not only io.EOF: a closed
// stdin or detached tty will never produce a different answer later, and
// treating only io.EOF as exhausted would spin promptRetryOrCancel forever.
func readLine(reader *bufio.Reader) (line string, unreadable bool) {
	line, err := reader.ReadString('\n')
	return line, err != nil
}

// promptRetryOrCancel explains why the user's last answer was rejected and
// asks whether to retry that same field or cancel the whole authoring
// session. It returns true when the user's reply is "cancel"
// (case-insensitive), or when the input is exhausted (EOF): an exhausted
// reader can never supply a different answer on a later retry, so treating
// it as anything but cancellation would spin the caller's prompt loop
// forever. Any other reply -- including "retry" -- is treated as a request
// to retry, so the caller's own prompt loop asks the field again.

// promptForLayers collects named layers and their prefixes, one at a time,
// until the user leaves a layer name blank. It never suggests a name or a
// prefix on the user's behalf: every layer in the returned slice came from
// the user's own typed answers. Each candidate is checked with the same
// name-uniqueness and prefix-overlap rules the frozen project-config schema
// itself enforces (validateProjectConfigLayers), so a mistake is caught and
// explained here rather than deferred to a later validation pass.

// parseRootSelection turns one line of user input into an ordered,
// deduplicated root list. A token that parses as a 1-based index into
// discoveredRoots resolves to that root; any other non-empty token is taken
// as a literal path exactly as typed. An out-of-range numeric token is
// rejected with an explanatory error rather than silently dropped: dropping
// it while keeping the rest of a valid answer would leave the resolved
// selection non-empty and so pass validateRootSelection unnoticed, meaning
// the customer's typo (or a stale suggestion list) silently selects fewer
// roots than they asked for with no indication anything was wrong.
