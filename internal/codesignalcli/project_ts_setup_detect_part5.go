package codesignalcli

import (
	"strings"
)

// bunfigUnverifiableHazard fails closed on a file this reader cannot claim to
// have understood, rather than on a redirect it recognized.
func bunfigUnverifiableHazard(data []byte) string {
	const unverifiable = "committed bunfig.toml could not be verified to leave package resolution unredirected"

	if strings.Contains(string(data), `\`) {
		return unverifiable
	}
	lower := strings.ToLower(string(data))
	for _, keyword := range []string{"install", "registry", "scopes", "cache"} {
		if strings.Contains(lower, keyword) {
			return unverifiable
		}
	}
	return ""
}

// reconcilePackageManagerDetection is the single cross-check rule shared by
// the worktree and snapshot detection paths so they cannot diverge (SA-280-012).
func reconcilePackageManagerDetection(fieldKind, fieldPin string, fieldOK bool, lockKind string, lockOK, lockAmbiguous bool) (packageManagerDetection, bool) {
	if lockAmbiguous || (fieldOK && lockOK && fieldKind != lockKind) {
		return packageManagerDetection{ambiguous: true}, true
	}
	switch {
	case fieldOK:
		return packageManagerDetection{kind: fieldKind, pin: fieldPin}, true
	case lockOK:
		return packageManagerDetection{kind: lockKind}, true
	default:
		return packageManagerDetection{}, false
	}
}

// detectNpmrcHazard reports a hazard detail for a committed .npmrc that
// redirects the registry, re-enables lifecycle scripts, or overrides the
// script shell -- npm's full Hazards column (SA-280-012).
func detectNpmrcHazard(root string) string {
	return scanNpmrcLines(root, func(key, value string) string {
		switch {
		case isNpmrcRegistryKey(key):
			return "committed .npmrc redirects the package registry (" + key + "=" + value + ")"
		case key == "ignore-scripts":
			if !strings.EqualFold(value, "true") {
				return "committed .npmrc re-enables lifecycle scripts (ignore-scripts=" + value + ")"
			}
		case key == "script-shell":
			return "committed .npmrc overrides the lifecycle script shell (script-shell=" + value + ")"
		}
		return ""
	})
}

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
