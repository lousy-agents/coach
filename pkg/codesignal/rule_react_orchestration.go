package codesignal

import (
	"regexp"
	"sort"
	"strconv"

	"github.com/lousy-agents/coach/pkg/semantics"
)

const reactOrchestrationWhyItMatters = "This component coordinates several independently changing UI concerns and feature panels. This is a refactoring opportunity, not a functional defect: changes to one workflow can unintentionally affect another, and the component is harder to test or extract safely."

const reactOrchestrationRecommendation = "Preserve the public component boundary. First extract one cohesive workspace or panel, then move only its local interaction state and derived data into that module or a dedicated hook. Keep cross-workspace state in the parent. Do not split by line count; derive explicit state-transition actions before moving state."

const (
	reactStateDomainMinUnique      = 3
	reactStateDomainMinTransitions = 1
	reactStateDomainMinBranches    = 3
)

// reactStateDomainRules is a closed, order-sensitive table of state-binding
// name patterns to domain ids. The first matching pattern wins, so more
// specific domains (e.g. navigation) must precede broader ones (e.g.
// selection) that would otherwise also match.
var reactStateDomainRules = []struct {
	domain  string
	pattern *regexp.Regexp
}{
	{"navigation", regexp.MustCompile(`(?i)^(active(view|tab|panel|section|page)|selected(view|tab)|current(view|tab)|view|tab|route|screen)$`)},
	{"selection", regexp.MustCompile(`(?i)(selected|selection|checked|highlighted)`)},
	{"hover_focus", regexp.MustCompile(`(?i)(hover|focused|focusable|^focus$)`)},
	{"filtering", regexp.MustCompile(`(?i)(filter|search|query)`)},
	{"pagination", regexp.MustCompile(`(?i)(pageindex|pagenumber|^page$|offset|cursor|pagelimit|^limit$)`)},
	{"sorting", regexp.MustCompile(`(?i)(sort|orderby)`)},
	{"modal", regexp.MustCompile(`(?i)(modal|dialog|drawer|expanded|collapsed|isopen|^open$|opened|isvisible|^visible$|^show$)`)},
	{"loading_error", regexp.MustCompile(`(?i)(loading|pending|iserror|^error$|^err$|fetchstatus|^status$)`)},
	{"other", regexp.MustCompile(`.*`)},
}

// classifyStateDomain maps a useState binding name to its domain id using
// reactStateDomainRules, first match wins.
func classifyStateDomain(binding string) string {
	for _, rule := range reactStateDomainRules {
		if rule.pattern.MatchString(binding) {
			return rule.domain
		}
	}
	return "other"
}

// uniqueStateDomains returns the sorted, deduplicated set of domain ids
// across record's UseState bindings.
func uniqueStateDomains(record semantics.ReactComponentFacts) []string {
	seen := make(map[string]bool, len(record.UseState))
	for _, binding := range record.UseState {
		seen[classifyStateDomain(binding.Binding)] = true
	}
	domains := make([]string, 0, len(seen))
	for domain := range seen {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	return domains
}

// signalsFromReactOrchestration derives structure.react_component_orchestration_density
// signals from per-component semantics.ReactComponentFacts records. A
// signal is emitted only when all required Emission Rule criteria hold and
// at least one supporting predicate holds; otherwise the record is silent.
func signalsFromReactOrchestration(path string, records []semantics.ReactComponentFacts) []Signal {
	var signals []Signal
	for _, rec := range records {
		if !reactOrchestrationRequiredMet(rec) || !reactOrchestrationSupportingMet(rec) {
			continue
		}
		signals = append(signals, newReactOrchestrationSignal(path, rec))
	}
	return signals
}

func reactOrchestrationRequiredMet(rec semantics.ReactComponentFacts) bool {
	if len(uniqueStateDomains(rec)) < reactStateDomainMinUnique {
		return false
	}
	if len(rec.CoordinatedTransitions) < reactStateDomainMinTransitions {
		return false
	}
	if len(rec.WorkspaceBranches) < reactStateDomainMinBranches {
		return false
	}
	return true
}

func reactOrchestrationSupportingMet(rec semantics.ReactComponentFacts) bool {
	for _, transition := range rec.CoordinatedTransitions {
		if transition.Kind == "effect" && len(transition.UpdatedBindings) >= 2 {
			return true
		}
		if len(transition.UpdatedBindings) >= 3 {
			return true
		}
	}
	if len(rec.ImperativeUI) >= 1 {
		return true
	}
	if len(rec.SharedPanelDeps) >= 1 {
		return true
	}
	return false
}

func newReactOrchestrationSignal(path string, rec semantics.ReactComponentFacts) Signal {
	return Signal{
		RuleID:         "structure.react_component_orchestration_density",
		RuleVersion:    "1",
		Kind:           "react_component_orchestration_density",
		Category:       "structure",
		Severity:       "medium",
		Confidence:     "high",
		Path:           path,
		Subject:        rec.Name,
		Location:       rec.Location,
		Evidence:       reactOrchestrationEvidence(rec),
		WhyItMatters:   reactOrchestrationWhyItMatters,
		Recommendation: reactOrchestrationRecommendation,
		Provenance: Provenance{
			Producer: "codesignal",
		},
	}
}

func reactOrchestrationEvidence(rec semantics.ReactComponentFacts) string {
	return "domains=" + strconv.Itoa(len(uniqueStateDomains(rec))) +
		";states=" + strconv.Itoa(len(rec.UseState)) +
		";transitions=" + strconv.Itoa(len(rec.CoordinatedTransitions)) +
		";branches=" + strconv.Itoa(len(rec.WorkspaceBranches)) +
		";imperative=" + strconv.Itoa(len(rec.ImperativeUI)) +
		";shared=" + strconv.Itoa(len(rec.SharedPanelDeps)) +
		";client=" + rec.ClientKind
}
