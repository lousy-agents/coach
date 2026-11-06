package projectreadiness

type State string

const (
	Pass       State = "pass"
	Fail       State = "fail"
	NotChecked State = "not_checked"
)

// WarnCompilerDeclarationMismatch is the limit-class warning when a
// selected root's manifest declares a typescript version other than the
// compiler the scan will use. A winning non-project origin warns for any
// differing declaration (range or exact); a winning project origin warns
// only for a stale exact pin -- a range is never warned about, and range
// satisfaction is never evaluated. It never appears in gaps[] and carries
// no next action: Coach never edits manifests.
const WarnCompilerDeclarationMismatch = "compiler_declaration_mismatch"

type RootFinding struct {
	Root    string `json:"root"`
	Version string `json:"version,omitempty"`
}

type OriginFinding struct {
	Origin string
	Class  string
}

type DeclarationMismatch struct {
	Root     string
	Declared string
}

// Check is one entry in Checks. Which optional fields
// accompany which code is the frozen compiler-check contract, pinned by
// cmd/coach's aggregation acceptance table; the json:"-" fields never
// serialize and reach the customer as rendered text only. Kind and Origin
// are additive and omitempty, populated only on Runtime and PackageManager
// (PackageManager's Kind names the detected manager: npm, pnpm, bun, or
// yarn). PinnedVersion is PackageManager's alone: it records package.json's
// packageManager pin, which is reported but never classified against, since
// the frozen adapter rows run whichever binary PATH resolves (see
// checkPackageManager).
type Check struct {
	State             State         `json:"state"`
	Code              string        `json:"code,omitempty"`
	Kind              string        `json:"kind,omitempty"`
	Version           string        `json:"version,omitempty"`
	ExpectedVersion   string        `json:"expected_version,omitempty"`
	FoundVersion      string        `json:"found_version,omitempty"`
	PinnedVersion     string        `json:"pinned_version,omitempty"`
	SupportedVersions []string      `json:"supported_versions,omitempty"`
	RootFindings      []RootFinding `json:"root_findings,omitempty"`
	Origin            string        `json:"origin,omitempty"`
	Detail            string        `json:"detail,omitempty"`
	// DecidedOnDisk is set when the working tree, not the analyzed revision,
	// produced this result. A change to the file it was decided from is then not
	// one the result ignored.
	DecidedOnDisk bool `json:"-"`
	// Remedy is the way out of a policy gap this check found. It is the zero
	// value, RemedyNone, for every check that is not a policy gap.
	Remedy PolicyRemedy `json:"-"`
	// Cause is, for a policy gap no remedy fixes, the reason in the words the
	// scan prints for the same policy, so a caller that cannot print the scan's own
	// error can still say why. It is empty for every other check.
	Cause string `json:"-"`
	// PolicyCause is the same reason as a stable word, for the gap that this check
	// becomes. It is empty whenever a remedy names what to do.
	PolicyCause PolicyCause `json:"-"`

	DeclaredVersion   string          `json:"-"`
	DeclarationOrigin string          `json:"-"`
	OriginFindings    []OriginFinding `json:"-"`

	DeclarationMismatches []DeclarationMismatch `json:"-"`
}

type Checks struct {
	ProjectShape   Check `json:"project_shape"`
	Policy         Check `json:"policy"`
	Node           Check `json:"node"`
	Runtime        Check `json:"runtime"`
	Compiler       Check `json:"compiler"`
	PackageManager Check `json:"package_manager"`
}
