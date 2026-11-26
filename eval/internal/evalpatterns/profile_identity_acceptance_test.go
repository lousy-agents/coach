package evalpatterns_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

func committedCoachSelf() []byte {
	GinkgoHelper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "patterns", "profiles", "coach-self.profile.json"))
	Expect(err).NotTo(HaveOccurred())
	return raw
}

// withOneMoreLayer is raw with one layer count raised and its identity
// left as it was.
func withOneMoreLayer(raw []byte) []byte {
	GinkgoHelper()
	var profile map[string]any
	Expect(json.Unmarshal(raw, &profile)).To(Succeed())
	layers := nested(profile, "dimensions")["layers"].(map[string]any)
	Expect(layers).NotTo(BeEmpty())
	for value, count := range layers {
		layers[value] = count.(float64) + 1
		break
	}
	edited, err := json.Marshal(profile)
	Expect(err).NotTo(HaveOccurred())
	return edited
}

var _ = Describe("a profile's identity (AC-5.1)", func() {
	It("verifies for the committed coach-self profile and the extractor's own test profile", func() {
		Expect(evalpatterns.VerifyProfileIdentity(committedCoachSelf())).To(Succeed())
		testdata, err := os.ReadFile(filepath.Join("testdata", "coach-self.profile.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(evalpatterns.VerifyProfileIdentity(testdata)).To(Succeed())
	})

	When("a count is edited and the identity is not", func() {
		It("fails, naming the identity the profile carries and the one its contents hash to", func() {
			raw := committedCoachSelf()
			var carried struct {
				Identity string `json:"identity"`
			}
			Expect(json.Unmarshal(raw, &carried)).To(Succeed())

			err := evalpatterns.VerifyProfileIdentity(withOneMoreLayer(raw))

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(carried.Identity))
			Expect(err.Error()).To(MatchRegexp(`hash to [0-9a-f]{64}`))
		})
	})
})
