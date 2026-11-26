package antipatternfixtures

// Finding is one finding a fixture's bad variant expects, as #871's
// expect.json writes it.
type Finding struct {
	RuleID  string `json:"rule_id"`
	Path    string `json:"path"`
	Subject string `json:"subject"`
}

// Template is one #871 catalogue pattern that a profile statistic scales.
// Its expectations are its own declaration, computed from the value it is
// scaled to, never from what Coach reports (design §6).
type Template struct {
	ID, Name, Status string
	observe          func(profile) (observation, bool)
	// Bad and Good are a variant's files, keyed by slash path, for the
	// value the template is scaled to.
	Bad, Good func(value int) map[string]string
	// Expect is the bad variant's findings; nil for a pattern Coach has no
	// rule for, whose status is not_covered.
	Expect func(value int) []Finding
	// Padding is the files a template of layers adds to both variants to
	// hold the observed packages per layer, given the value and the packages.
	Padding func(value, packages int) map[string]string
}

// Templates are the #871 patterns a syntax-only profile can observe. The
// others (god-struct, check-then-act, global-mutable-state,
// hidden-input-mutation, create-after-missing-check and
// bypass-without-required-layer) have no shape in a profile.
func Templates() []Template {
	return []Template{arrowCode(), constructorDoesRealWork(), dependencyRuleBreach(), godFile(), skippedLayer(), spaghettiConditional()}
}

func goMod(module string) string { return "module example.com/" + module + "\n\ngo 1.26.0\n" }
