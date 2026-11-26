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
