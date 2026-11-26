package antipatternfixtures_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// expectLayerCount requires both layer fixtures to hold want layers, and to say so.
func expectLayerCount(out string, want int) {
	GinkgoHelper()
	for _, id := range layerIDs {
		Expect(policyOf(out, id, "bad").Layers).To(HaveLen(want))
		Expect(readExpect(out, id).Note).To(ContainSubstring("layer count " + itoa(want)))
	}
}

// expectPackagesPerLayer requires every layer of every variant to hold want packages, and the note to say so, and say
// what it was capped from when it was.
func expectPackagesPerLayer(out string, layers, want, from int) {
	GinkgoHelper()
	for _, id := range layerIDs {
		for _, variant := range variants {
			expectEveryRoleHolds(packageDirs(out, id, variant), layers, want)
		}
		expectCapNote(readExpect(out, id).Note, want, from)
	}
}

func expectEveryRoleHolds(dirs map[string][]string, layers, want int) {
	GinkgoHelper()
	Expect(dirs).To(HaveLen(layers))
	for _, role := range generatorRoles[layers] {
		Expect(dirs[role]).To(HaveLen(want), role)
	}
}

func expectCapNote(note string, want, from int) {
	GinkgoHelper()
	Expect(note).To(ContainSubstring("packages per layer " + itoa(want)))
	if from == want {
		Expect(note).NotTo(ContainSubstring("capped"))
		return
	}
	Expect(note).To(ContainSubstring("capped at " + itoa(want) + " from " + itoa(from)))
}

// innerToOuter is every pair of layers of the layer count as "inner>outer": the pairs the policy forbids.
func innerToOuter(roles []string) []string {
	var pairs []string
	for outer := range roles {
		for _, inner := range roles[outer+1:] {
			pairs = append(pairs, inner+">"+roles[outer])
		}
	}
	return pairs
}

// expectOrderedByForbiddenPairs requires each layer fixture's policy to name the layers outermost first, with
// internal/<role> as the prefix, and to forbid exactly the inner-to-outer pairs.
func expectOrderedByForbiddenPairs(out string, layers int) {
	GinkgoHelper()
	for _, id := range layerIDs {
		for _, variant := range variants {
			expectOrderedPolicy(policyOf(out, id, variant), layers)
		}
	}
}

func expectOrderedPolicy(policy policyFile, layers int) {
	GinkgoHelper()
	var names, forbidden []string
	for _, layer := range policy.Layers {
		names = append(names, layer.Name)
		Expect(layer.Prefixes).To(Equal([]string{"internal/" + layer.Name}))
	}
	for _, pair := range policy.ForbiddenImports {
		forbidden = append(forbidden, pair.From+">"+pair.To)
	}
	Expect(names).To(Equal(generatorRoles[layers]))
	Expect(forbidden).To(ConsistOf(innerToOuter(generatorRoles[layers])))
}

// expectDeclaredFindings requires each layer fixture's expectations to be the template's declaration.
func expectDeclaredFindings(out string) {
	GinkgoHelper()
	Expect(badFindings(readExpect(out, "dependency-rule-breach"))).To(Equal([]finding{{
		RuleID: "architecture.layer_violation", Path: "internal/store/orders/reach.go",
		Subject: "architecture.layer_violation:internal/store/orders->internal/api/orders"}}))
	Expect(badFindings(readExpect(out, "skipped-layer"))).To(Equal([]finding{{
		RuleID: "architecture.layer_bypass", Path: "internal/api/orders/handle.go",
		Subject: "architecture.layer_bypass:service:example.com/layers/internal/api/orders.Handle->(*database/sql.DB).Query"}}))
	for _, id := range layerIDs {
		expectFixtureMetadata(readExpect(out, id), id)
	}
}

func expectFixtureMetadata(expect expectFile, id string) {
	GinkgoHelper()
	Expect(expect.Status).To(Equal("covered"))
	Expect(string(expect.Expect.Good)).To(Equal(`"none"`))
	Expect(expect).To(SatisfyAll(
		HaveField("ID", id), HaveField("Seed", "profile:"+profileIdentity),
		HaveField("Source", SatisfyAll(ContainSubstring("#871"), ContainSubstring(id)))))
}
