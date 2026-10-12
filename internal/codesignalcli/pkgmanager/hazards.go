package pkgmanager

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// detectBunfigHazard reports a hazard detail for a committed bunfig.toml
// that carries any unverified [install]-namespaced configuration: a registry
// redirect (globally under [install] or for a specific scope either under
// [install.scopes] or via [install]'s own "scopes" table), or an
// [install.cache] "dir" redirect -- verified empirically against real Bun
// 1.3.11 that `bun install --frozen-lockfile --ignore-scripts` still
// requests packages from a bunfig.toml-configured registry rather than the
// default one (globally, per scope under [install.scopes], and per scope via
// [install]'s "scopes" key), and, more severely, that a redirected
// [install.cache] dir makes the same frozen install silently succeed with
// attacker-controlled file content from that directory -- no network fetch,
// no integrity-check failure, and no lifecycle script needed. bunfig.toml's
// "preload" entry was verified empirically not to fire on `bun install` at all
// (only on `bun run`/the bun runtime), so it is not treated as a hazard
// here. Like detectNpmrcHazard, a bunfig.toml that exists but cannot be read
// is itself a hazard, distinct from no bunfig.toml existing at all. This is
// a narrow, section-and-key scan for the hazardous forms above (registry and
// scopes only; cache is left to the backstop below for its detail text),
// not a general TOML parser -- consistent with detectNpmrcHazard's own
// narrow ini scan of .npmrc. Because it is not a real TOML parser, it cannot
// recognize every legal spelling that redirects install behavior (a header
// followed by a trailing comment, a leading UTF-8 BOM, or an inline table
// were all verified empirically against real Bun 1.3.11 to be honored while
// evading the structured scan below) -- and a keyword-by-keyword backstop
// will always be one Bun release behind the next [install]-namespaced key.
// So the fail-closed backstop matches a union of "install", "registry",
// "scopes", and "cache" anywhere in the file (case-insensitive), at the cost
// of over-refusing rather than under-refusing. Matching only "install" was
// tried and found insufficient on its own: real Bun 1.3.11 decodes TOML
// escape sequences -- \uXXXX, \xHH, and octal \NNN alike -- inside a quoted
// table name or a quoted key (["insta\x6Cl"]/"regist\x72y" decodes the same
// way ["install"]/"registry" would), which can strip every one of these
// keywords from a substring scan while the redirect -- including into
// [install.cache], the cache-poisoning vector above -- still takes effect.
// Rather than keep enumerating individual escape spellings, any backslash
// anywhere in the file is treated as its own fail-closed ground for refusal:
// every TOML escape mechanism, present or future, requires a backslash to
// invoke, and a legitimate bunfig.toml's paths and settings have no
// legitimate reason to contain one. Together these narrow the gap but do not
// close it for every possible future [install]-namespaced key or every
// other TOML encoding trick; the structured cases above remain only for
// their more specific, actionable detail text.
func detectBunfigHazard(root string) string {
	data, detail, present := readCommittedBunfig(root)
	if !present {
		return detail
	}
	if hazard := bunfigRedirectHazard(data); hazard != "" {
		return hazard
	}
	return bunfigUnverifiableHazard(data)
}

// readCommittedBunfig reads root's bunfig.toml. present is false both when
// there is genuinely no file (detail "": nothing to be hazardous) and when
// one exists but cannot be read (detail names the hazard) -- the fail-closed
// half AC-SET-12 requires, since an unreadable config is not a safe one.
func readCommittedBunfig(root string) (data []byte, detail string, present bool) {
	path := filepath.Join(root, "bunfig.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if _, statErr := os.Lstat(path); errors.Is(statErr, fs.ErrNotExist) {
			return nil, "", false
		}
		return nil, "committed bunfig.toml could not be read", false
	}
	return bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF")), "", true
}

// DetectPackageManagerHazard reports a non-empty detail describing a
// repository-controlled configuration hazard from kind's Hazards column
// (SA-280-012), or "" if none. The lockfile-readability precondition itself
// is kind-agnostic; see requireReadableLockfile.
func DetectPackageManagerHazard(root, kind string) string {
	switch kind {
	case KindNPM:
		return detectNpmrcHazard(root)
	case KindPNPM:
		return detectNpmrcRegistryHazard(root)
	case KindBun:
		// Bun resolves its install registry from a committed .npmrc the same
		// way npm/pnpm do, on top of bunfig.toml rather than instead of it
		// (verified empirically against real Bun 1.3.11: `bun install
		// --ignore-scripts` still fails with ConnectionRefused against a
		// .npmrc-redirected host with no bunfig.toml present at all) -- so
		// both must be checked, not just bunfig.toml.
		if detail := detectNpmrcRegistryHazard(root); detail != "" {
			return detail
		}
		return detectBunfigHazard(root)
	default:
		return ""
	}
}
