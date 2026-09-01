package codesignalcli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// AuthoringResult accumulates one guided policy-authoring session's answers,
// in the order they are collected: roots, then layers, then forbidden layer
// pairs, then the optional required intermediary layer.
type AuthoringResult struct {
	// Roots holds the directories the user explicitly selected or typed,
	// in the order they named them. It is nil, not a copy of a discovered
	// suggestion, when the user gave no answer -- selection always requires
	// the user's own input.
	Roots []string

	// Layers holds the named layers the user defined, each with the
	// prefixes they typed for it, in the order they were entered. It is
	// nil if the user finished the layer stage without defining any.
	Layers []projectConfigLayer

	// ForbiddenImports holds the forbidden layer-import pairs the user
	// defined, in the order they were entered. It is nil if the user
	// finished that stage without defining any.
	ForbiddenImports []projectForbiddenImport

	// RequiredLayer names the required intermediary layer the user chose,
	// or is empty if they left it unset.
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

// AuthorProjectConfig runs the guided project-config authoring prompt
// sequence over in/out rather than a real terminal, so it is testable
// without a pty. discovered is supplied by the caller; this function never
// runs discovery itself. out carries every prompt, explanation, and the
// coverage preview -- the whole human-facing transcript -- and is kept
// strictly separate from candidateOut, which receives only the approved
// candidate document itself when outputSet is false. Callers must bind out
// to a stream the human can see even when they redirect whatever stream
// candidateOut is bound to (e.g. out to stderr, candidateOut to stdout):
// mixing the two into one writer would make a captured candidate document
// unparseable, and would leave the transcript invisible if that same
// stream were redirected. dir, outputPath, and outputSet are used only once
// the user approves the candidate: dir is the repository root the write is
// confined to, and outputPath/outputSet select between a create-only write
// (outputSet true) and emitting the candidate document to candidateOut
// (outputSet false). None of the three plays any part in field collection,
// the coverage preview, or the approval gate itself.
//
// This function only ever suggests roots DiscoverTSRoots already found; it
// never preselects or infers one on the user's behalf, and it never groups,
// names, or otherwise proposes an architectural layer boundary -- that
// collection step belongs to a later stage, not this one.
func AuthorProjectConfig(dir string, in io.Reader, out io.Writer, candidateOut io.Writer, discovered projectmodel.TSRootDiscoveryResult, outputPath string, outputSet bool) AuthoringResult {
	reader := bufio.NewReader(in)
	roots, cancelled := promptForRoots(out, reader, discovered)
	if cancelled {
		return AuthoringResult{Cancelled: true}
	}

	layers, cancelled := promptForLayers(out, reader)
	if cancelled {
		return AuthoringResult{Roots: roots, Layers: layers, Cancelled: true}
	}

	forbidden, cancelled := promptForForbiddenImports(out, reader, layers)
	if cancelled {
		return AuthoringResult{Roots: roots, Layers: layers, ForbiddenImports: forbidden, Cancelled: true}
	}

	requiredLayer, cancelled := promptForRequiredLayer(out, reader, layers)
	if cancelled {
		return AuthoringResult{Roots: roots, Layers: layers, ForbiddenImports: forbidden, Cancelled: true}
	}

	approved := promptForApproval(out, reader, discovered, roots, layers, forbidden, requiredLayer)
	result := AuthoringResult{Roots: roots, Layers: layers, ForbiddenImports: forbidden, RequiredLayer: requiredLayer, Approved: approved}
	if !approved {
		return result
	}
	return finalizeApprovedCandidate(result, dir, candidateOut, outputPath, outputSet)
}

// buildApprovedCandidate renders the collected fields as the schema-1
// project-config document and runs it through parseProjectConfig -- the same
// validator LoadProjectConfig applies to a committed --project-config file
// -- before treating it as valid. source_sink_pack is never populated: it is
// a reserved field this feature must not touch.
func buildApprovedCandidate(roots []string, layers []projectConfigLayer, forbidden []projectForbiddenImport, requiredLayer string) ([]byte, error) {
	candidate := projectConfig{
		SchemaVersion:    "1",
		Roots:            roots,
		Layers:           layers,
		ForbiddenImports: forbidden,
		RequiredLayer:    requiredLayer,
	}
	data, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if _, err := parseProjectConfig(data); err != nil {
		return nil, err
	}
	return data, nil
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
	answer, _ := readLine(reader)
	return strings.EqualFold(strings.TrimSpace(answer), "approve")
}

// printCandidateSummary prints every field collected so far, legibly, so the
// user reviews the complete candidate before being asked to approve it.
func printCandidateSummary(out io.Writer, roots []string, layers []projectConfigLayer, forbidden []projectForbiddenImport, requiredLayer string) {
	fmt.Fprintln(out, "Candidate project config:")
	fmt.Fprintf(out, "  roots: %s\n", formatStringList(roots))
	if len(layers) == 0 {
		fmt.Fprintln(out, "  layers: (none)")
	} else {
		fmt.Fprintln(out, "  layers:")
		for _, layer := range layers {
			fmt.Fprintf(out, "    - %s: %s\n", layer.Name, strings.Join(layer.Prefixes, ", "))
		}
	}
	if len(forbidden) == 0 {
		fmt.Fprintln(out, "  forbidden_imports: (none)")
	} else {
		fmt.Fprintln(out, "  forbidden_imports:")
		for _, pair := range forbidden {
			fmt.Fprintf(out, "    - %s -> %s\n", pair.From, pair.To)
		}
	}
	if requiredLayer == "" {
		fmt.Fprintln(out, "  required_layer: (none)")
	} else {
		fmt.Fprintf(out, "  required_layer: %s\n", requiredLayer)
	}
}

// printCoveragePreview prints, for each declared layer, which of the
// discovered directories (both DiscoverTSRoots' Roots and Candidates) its
// prefixes match, and then a final line naming every discovered directory
// that no declared layer matches. This is the preview the approval gate is
// there to protect: it shows what an eventual real analysis run would and
// would not classify under the candidate policy.
func printCoveragePreview(out io.Writer, discovered projectmodel.TSRootDiscoveryResult, layers []projectConfigLayer) {
	dirs := discoveredDirectories(discovered)

	fmt.Fprintln(out, "Coverage preview:")
	for _, layer := range layers {
		matched := matchingDiscoveredDirectories(layer, dirs)
		fmt.Fprintf(out, "  layer %q (prefixes: %s) matches: %s\n", layer.Name, strings.Join(layer.Prefixes, ", "), formatStringList(matched))
	}

	uncovered := uncoveredDiscoveredDirectories(dirs, layers)
	fmt.Fprintf(out, "  discovered directories no declared layer matches: %s\n", formatStringList(uncovered))
}

// discoveredDirectories returns discovered.Roots and discovered.Candidates
// combined into one ordered, deduplicated list -- the coverage preview
// treats every directory DiscoverTSRoots found as something the eventual
// policy should account for, not only the ones it called out as tsconfig
// roots.
func discoveredDirectories(discovered projectmodel.TSRootDiscoveryResult) []string {
	var all []string
	seen := map[string]bool{}
	for _, group := range [][]string{discovered.Roots, discovered.Candidates} {
		for _, dir := range group {
			if !seen[dir] {
				seen[dir] = true
				all = append(all, dir)
			}
		}
	}
	return all
}

// directoryHasPrefix reports whether prefix matches dir the same way a real
// layer-violation evaluation would (pkg/codesignal/rule_layer_violation_match.go's
// layerContainsDir): dir equals prefix, dir is nested under prefix, or prefix
// is ".", the universal repository-root ancestor.
func directoryHasPrefix(dir, prefix string) bool {
	return prefix == "." || dir == prefix || strings.HasPrefix(dir, prefix+"/")
}

func layerMatchesDirectory(layer projectConfigLayer, dir string) bool {
	for _, prefix := range layer.Prefixes {
		if directoryHasPrefix(dir, prefix) {
			return true
		}
	}
	return false
}

func matchingDiscoveredDirectories(layer projectConfigLayer, dirs []string) []string {
	var matched []string
	for _, dir := range dirs {
		if layerMatchesDirectory(layer, dir) {
			matched = append(matched, dir)
		}
	}
	return matched
}

func uncoveredDiscoveredDirectories(dirs []string, layers []projectConfigLayer) []string {
	var uncovered []string
	for _, dir := range dirs {
		covered := false
		for _, layer := range layers {
			if layerMatchesDirectory(layer, dir) {
				covered = true
				break
			}
		}
		if !covered {
			uncovered = append(uncovered, dir)
		}
	}
	return uncovered
}

// formatStringList renders items as a comma-separated list, or "(none)" when
// there are none, so an empty match/uncovered set never prints as a blank
// line the user might mistake for missing output.
func formatStringList(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
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
func promptForRoots(out io.Writer, reader *bufio.Reader, discovered projectmodel.TSRootDiscoveryResult) (roots []string, cancelled bool) {
	printDiscoveredRoots(out, discovered)

	for {
		fmt.Fprintln(out, "Select the roots to include: enter comma-separated numbers from the list above and/or directory paths, then press Enter. At least one repository-relative root is required.")
		fmt.Fprint(out, "> ")

		answer, _ := readLine(reader)
		selected, parseErr := parseRootSelection(answer, discovered.Roots)
		if parseErr == nil {
			parseErr = validateRootSelection(selected)
		}
		if parseErr != nil {
			if promptRetryOrCancel(out, reader, parseErr.Error()) {
				return nil, true
			}
			continue
		}
		return selected, false
	}
}

// printDiscoveredRoots prints DiscoverTSRoots' own findings as a suggestion,
// never as a preselection: promptForRoots always reads the user's own answer
// afterward regardless of what is printed here.
func printDiscoveredRoots(out io.Writer, discovered projectmodel.TSRootDiscoveryResult) {
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

// validateRootSelection applies validateProjectConfigRoots' own per-entry
// rule (validateProjectConfigDirectory) plus its non-empty requirement to one
// candidate root selection, so an author sees the same rejection at
// collection time that the final schema validation would otherwise only
// apply once every remaining stage had already been answered.
func validateRootSelection(selected []string) error {
	if len(selected) == 0 {
		return fmt.Errorf("at least one repository-relative root must be selected")
	}
	if len(selected) > maxProjectConfigRoots {
		return fmt.Errorf("roots exceed budget of %d entries", maxProjectConfigRoots)
	}
	for _, root := range selected {
		if err := validateProjectConfigDirectory(root); err != nil {
			return fmt.Errorf("root %q: %s", root, err)
		}
	}
	return nil
}

// readLine reads and returns one line from reader, including its trailing
// newline if present, and whether the read ended because input is
// exhausted. A missing trailing newline before EOF is still returned as the
// line's content, matching bufio.Reader.ReadString's own contract. exhausted
// is set on ANY read error, not only io.EOF: a real reader can fail with
// something other than a clean end-of-input (a closed stdin file
// descriptor, a detached tty, a reset network stream), and like io.EOF it
// will never produce a different answer on a later read. Recognizing only
// io.EOF here would let promptRetryOrCancel keep retrying forever against a
// reader that can only ever error again.
func readLine(reader *bufio.Reader) (line string, exhausted bool) {
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
func promptRetryOrCancel(out io.Writer, reader *bufio.Reader, explanation string) bool {
	fmt.Fprintf(out, "That answer is invalid: %s\n", explanation)
	fmt.Fprintln(out, "Type 'retry' to try again, or 'cancel' to cancel authoring:")
	fmt.Fprint(out, "> ")
	reply, eof := readLine(reader)
	if eof {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(reply), "cancel")
}

// layerNameDeclared reports whether name matches an already-collected
// layer, used both to reject a duplicate layer name and to check that a
// forbidden-pair endpoint or required-layer answer names a real layer.
func layerNameDeclared(name string, layers []projectConfigLayer) bool {
	for _, layer := range layers {
		if layer.Name == name {
			return true
		}
	}
	return false
}

// promptForLayers collects named layers and their prefixes, one at a time,
// until the user leaves a layer name blank. It never suggests a name or a
// prefix on the user's behalf: every layer in the returned slice came from
// the user's own typed answers. Each candidate is checked with the same
// name-uniqueness and prefix-overlap rules the frozen project-config schema
// itself enforces (validateProjectConfigLayers), so a mistake is caught and
// explained here rather than deferred to a later validation pass.
func promptForLayers(out io.Writer, reader *bufio.Reader) (layers []projectConfigLayer, cancelled bool) {
	fmt.Fprintln(out, "Define named layers for architecture-boundary policy. Each layer needs a name and one or more repository-relative path prefixes.")
	for {
		name, done, cancelled := promptLayerName(out, reader, layers)
		if cancelled {
			return layers, true
		}
		if done {
			return layers, false
		}

		prefixes, cancelled := promptLayerPrefixes(out, reader, name, layers)
		if cancelled {
			return layers, true
		}
		layers = append(layers, projectConfigLayer{Name: name, Prefixes: prefixes})
	}
}

func promptLayerName(out io.Writer, reader *bufio.Reader, existing []projectConfigLayer) (name string, done, cancelled bool) {
	for {
		fmt.Fprintln(out, "Enter a layer name, or leave blank to finish defining layers:")
		fmt.Fprint(out, "> ")
		answer, _ := readLine(reader)
		answer = strings.TrimSpace(answer)
		if answer == "" {
			return "", true, false
		}
		if layerNameDeclared(answer, existing) {
			if promptRetryOrCancel(out, reader, fmt.Sprintf("layer name %q is already used", answer)) {
				return "", false, true
			}
			continue
		}
		return answer, false, false
	}
}

func promptLayerPrefixes(out io.Writer, reader *bufio.Reader, name string, existing []projectConfigLayer) (prefixes []string, cancelled bool) {
	for {
		fmt.Fprintf(out, "Enter comma-separated repository-relative path prefixes for layer %q:\n", name)
		fmt.Fprint(out, "> ")
		answer, _ := readLine(reader)
		candidate := splitTrimmedNonEmpty(answer, ",")
		if err := validateLayerPrefixCandidate(name, candidate, existing); err != nil {
			if promptRetryOrCancel(out, reader, err.Error()) {
				return nil, true
			}
			continue
		}
		return candidate, false
	}
}

// validateLayerPrefixCandidate applies validateProjectConfigLayers' own
// per-prefix rules -- non-empty, valid repository-relative directory syntax,
// unique and non-overlapping with every prefix already accepted -- to one
// new layer's candidate prefixes, so an author sees the same rejection here
// that the final schema validation would apply.
func validateLayerPrefixCandidate(name string, prefixes []string, existing []projectConfigLayer) error {
	if len(prefixes) == 0 {
		return fmt.Errorf("layer %q must contain at least one prefix", name)
	}
	var allPrefixes []string
	for _, layer := range existing {
		allPrefixes = append(allPrefixes, layer.Prefixes...)
	}
	for _, prefix := range prefixes {
		if err := validateProjectConfigDirectory(prefix); err != nil {
			return fmt.Errorf("prefix %q: %s", prefix, err)
		}
		allPrefixes = append(allPrefixes, prefix)
	}
	if len(allPrefixes) > maxProjectConfigLayerPrefixes {
		return fmt.Errorf("layer prefixes exceed budget of %d entries", maxProjectConfigLayerPrefixes)
	}
	if hasDuplicateOrOverlappingPaths(allPrefixes) {
		return fmt.Errorf("layer prefixes must be unique and non-overlapping across all layers")
	}
	return nil
}

// promptForForbiddenImports collects forbidden (from, to) layer-import
// pairs, one at a time, until the user leaves the source layer name blank.
// It applies validateProjectConfigForbiddenImports' own rules: both
// endpoints must name an already-declared layer, and each pair must be
// unique.
func promptForForbiddenImports(out io.Writer, reader *bufio.Reader, layers []projectConfigLayer) (forbidden []projectForbiddenImport, cancelled bool) {
	fmt.Fprintln(out, "Define forbidden layer-import pairs (a source layer that may not import a destination layer). Leave the source blank to finish.")
	for {
		from, to, done, cancelled := promptForbiddenPair(out, reader, layers, forbidden)
		if cancelled {
			return forbidden, true
		}
		if done {
			return forbidden, false
		}
		forbidden = append(forbidden, projectForbiddenImport{From: from, To: to})
	}
}

func promptForbiddenPair(out io.Writer, reader *bufio.Reader, layers []projectConfigLayer, existing []projectForbiddenImport) (from, to string, done, cancelled bool) {
	for {
		fmt.Fprintln(out, "Enter the source layer name for a forbidden import pair, or leave blank to finish:")
		fmt.Fprint(out, "> ")
		fromAnswer, _ := readLine(reader)
		fromAnswer = strings.TrimSpace(fromAnswer)
		if fromAnswer == "" {
			return "", "", true, false
		}

		fmt.Fprintln(out, "Enter the destination layer name that the source layer may not import:")
		fmt.Fprint(out, "> ")
		toAnswer, _ := readLine(reader)
		toAnswer = strings.TrimSpace(toAnswer)

		if err := validateForbiddenPairCandidate(fromAnswer, toAnswer, layers, existing); err != nil {
			if promptRetryOrCancel(out, reader, err.Error()) {
				return "", "", false, true
			}
			continue
		}
		return fromAnswer, toAnswer, false, false
	}
}

func validateForbiddenPairCandidate(from, to string, layers []projectConfigLayer, existing []projectForbiddenImport) error {
	if from == "" || to == "" {
		return fmt.Errorf("forbidden import pairs require a non-empty source and destination layer")
	}
	if !layerNameDeclared(from, layers) {
		return fmt.Errorf("forbidden import pair references undefined layer %q", from)
	}
	if !layerNameDeclared(to, layers) {
		return fmt.Errorf("forbidden import pair references undefined layer %q", to)
	}
	for _, pair := range existing {
		if pair.From == from && pair.To == to {
			return fmt.Errorf("forbidden import pairs must be unique")
		}
	}
	return nil
}

// promptForRequiredLayer collects the optional required intermediary layer.
// A blank answer leaves it unset; a non-blank answer must name an
// already-declared layer, matching validateProjectConfigCrossFields' rule
// for required_layer.
func promptForRequiredLayer(out io.Writer, reader *bufio.Reader, layers []projectConfigLayer) (requiredLayer string, cancelled bool) {
	for {
		fmt.Fprintln(out, "Enter the name of a required intermediary layer, or leave blank for none:")
		fmt.Fprint(out, "> ")
		answer, _ := readLine(reader)
		answer = strings.TrimSpace(answer)
		if answer == "" {
			return "", false
		}
		if !layerNameDeclared(answer, layers) {
			if promptRetryOrCancel(out, reader, fmt.Sprintf("required_layer references undefined layer %q", answer)) {
				return "", true
			}
			continue
		}
		return answer, false
	}
}

// splitTrimmedNonEmpty splits s on sep, trims surrounding whitespace from
// each piece, and drops empty pieces -- e.g. a trailing comma or repeated
// separators produce no spurious empty prefix.
func splitTrimmedNonEmpty(s, sep string) []string {
	var result []string
	for _, piece := range strings.Split(s, sep) {
		piece = strings.TrimSpace(piece)
		if piece != "" {
			result = append(result, piece)
		}
	}
	return result
}

// parseRootSelection turns one line of user input into an ordered,
// deduplicated root list. A token that parses as a 1-based index into
// discoveredRoots resolves to that root; any other non-empty token is taken
// as a literal path exactly as typed. An out-of-range numeric token is
// rejected with an explanatory error rather than silently dropped: dropping
// it while keeping the rest of a valid answer would leave the resolved
// selection non-empty and so pass validateRootSelection unnoticed, meaning
// the customer's typo (or a stale suggestion list) silently selects fewer
// roots than they asked for with no indication anything was wrong.
func parseRootSelection(answer string, discoveredRoots []string) ([]string, error) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return nil, nil
	}

	var selected []string
	seen := map[string]bool{}
	for _, token := range strings.Split(answer, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		root := token
		if idx, err := strconv.Atoi(token); err == nil {
			if idx < 1 || idx > len(discoveredRoots) {
				return nil, fmt.Errorf("%q is not a valid root number: only 1-%d are listed above", token, len(discoveredRoots))
			}
			root = discoveredRoots[idx-1]
		}
		if seen[root] {
			continue
		}
		seen[root] = true
		selected = append(selected, root)
	}
	return selected, nil
}
