package evalpatterns_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

func standInReference() evalpatterns.Reference {
	return evalpatterns.Reference{Label: "stand-in", SHA: sampleSHA, Language: "go", Licence: "MIT"}
}

func extract(root string) []byte {
	GinkgoHelper()
	raw, err := evalpatterns.ExtractProfile(standInReference(), root)
	Expect(err).NotTo(HaveOccurred())
	return raw
}

func decodeProfile(raw []byte) map[string]any {
	GinkgoHelper()
	var profile map[string]any
	Expect(json.Unmarshal(raw, &profile)).To(Succeed())
	return profile
}

var _ = Describe("extracting a profile from a checkout (#872 Task 2)", func() {
	When("the checkout is the stand-in tree", func() {
		It("records each measure of its production Go files, and nothing of its tests, testdata or vendor (AC-2.3, AC-2.5, AC-2.6)", func() {
			profile := decodeProfile(extract(standInTree()))

			Expect(profile["dimensions"]).To(Equal(map[string]any{
				"layers": map[string]any{},
				"entry":  map[string]any{"http_handler": 2.0, "method": 1.0, "function": 4.0},
				"depth":  map[string]any{},
				"trap":   nil, "field": nil, "direction": nil,
			}))
			Expect(profile["shapes"]).To(Equal(map[string]any{
				"nesting":        map[string]any{"0": 6.0, "1": 1.0, "4": 1.0},
				"cognitive":      map[string]any{"0": 6.0, "3": 1.0, "11": 1.0},
				"branch_density": map[string]any{"3": 1.0, "4": 1.0},
				"constructor":    map[string]any{"assigns_only": 1.0, "does_work": 1.0},
			}))
			Expect(profile["roles"]).To(Equal(map[string]any{}))
			Expect(profile["not_covered"]).To(Equal([]any{}))
		})

		It("carries the entry's label, SHA and licence, the extractor version and no other free text", func() {
			profile := decodeProfile(extract(standInTree()))

			Expect(profile["schema_version"]).To(Equal("1"))
			Expect(profile["label"]).To(Equal("stand-in"))
			Expect(profile["sha"]).To(Equal(sampleSHA))
			Expect(profile["licence"]).To(Equal("MIT"))
			Expect(profile["extractor_version"]).To(Equal(evalpatterns.ExtractorVersion))
		})

		It("validates against the committed schema (AC-1.4)", func() {
			Expect(evalpatterns.ValidateProfile(extract(standInTree()))).To(Succeed())
		})

		It("holds no identifier, path or module name of the stand-in (AC-1.3)", func() {
			raw := string(extract(standInTree()))

			for _, name := range []string{"ledger", "pick", "Server", "NewServer", "Arrow", "Pick", "Handle", "standin", "example.invalid", "apply.go", "net/http"} {
				Expect(raw).NotTo(ContainSubstring(name))
			}
		})

		It("writes canonical JSON: sorted keys, no insignificant whitespace, one trailing newline", func() {
			raw := extract(standInTree())

			Expect(raw).To(HaveSuffix("}\n"))
			canonical, err := json.Marshal(decodeProfile(raw))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(raw)).To(Equal(string(canonical) + "\n"))
		})

		It("sets the identity to the sha256 of the canonical JSON with identity omitted (AC-5.1)", func() {
			profile := decodeProfile(extract(standInTree()))
			identity := profile["identity"]
			delete(profile, "identity")
			canonical, err := json.Marshal(profile)
			Expect(err).NotTo(HaveOccurred())
			sum := sha256.Sum256(canonical)

			Expect(identity).To(Equal(hex.EncodeToString(sum[:])))
		})

		It("writes byte-identical profiles on two runs (AC-1.10)", func() {
			root := standInTree()

			Expect(extract(root)).To(Equal(extract(root)))
		})

		It("changes the identity when the licence changes, so every field is covered", func() {
			root := standInTree()
			other := standInReference()
			other.Licence = "Apache-2.0"
			raw, err := evalpatterns.ExtractProfile(other, root)
			Expect(err).NotTo(HaveOccurred())

			Expect(decodeProfile(raw)["identity"]).NotTo(Equal(decodeProfile(extract(root))["identity"]))
		})

		It("runs no go and no git command (AC-1.2)", func() {
			marker := filepath.Join(GinkgoT().TempDir(), "ran")
			plantShims(marker)

			extract(standInTree())

			Expect(marker).NotTo(BeAnExistingFile())
		})

		It("writes nothing into the checkout or anywhere else it was not asked to: no census or history record (AC-5.4)", func() {
			root := standInTree()
			before := treeListing(root)
			cwd := GinkgoT().TempDir()
			GinkgoT().Chdir(cwd)

			extract(root)

			Expect(treeListing(root)).To(Equal(before))
			Expect(os.ReadDir(cwd)).To(BeEmpty())
		})
	})

	When("a production Go file does not parse", func() {
		It("refuses the entry with exit 3, naming the label and the file (AC-1.9)", func() {
			root := standInTree()
			writeFile(filepath.Join(root, "pick", "broken.go"), "package pick\n\nfunc broken( {\n")

			_, err := evalpatterns.ExtractProfile(standInReference(), root)

			var refusal evalpatterns.Refusal
			Expect(errors.As(err, &refusal)).To(BeTrue(), "ExtractProfile returned %v", err)
			Expect(refusal.Exit).To(Equal(3))
			Expect(err.Error()).To(HavePrefix(`reference "stand-in": `))
			Expect(err.Error()).To(ContainSubstring("pick/broken.go"))
		})
	})

	When("the checkout directory's own name starts with _", func() {
		It("still reads it: only directories inside the checkout are ignored by name", func() {
			parent := GinkgoT().TempDir()
			root := filepath.Join(parent, "_checkout")
			Expect(os.Rename(standInTree(), root)).To(Succeed())

			Expect(decodeProfile(extract(root))["shapes"].(map[string]any)["branch_density"]).To(Equal(map[string]any{"3": 1.0, "4": 1.0}))
		})
	})

	When("the entry's label passes Check but not the profile schema", func() {
		It("refuses to return a profile that does not validate", func() {
			reference := standInReference()
			reference.Label = "Not A Path Segment"

			_, err := evalpatterns.ExtractProfile(reference, standInTree())

			Expect(err).To(MatchError(ContainSubstring("the profile does not validate")))
		})
	})

	When("the entry itself is refused", func() {
		It("refuses before reading the checkout", func() {
			reference := standInReference()
			reference.Licence = " "

			_, err := evalpatterns.ExtractProfile(reference, filepath.Join(GinkgoT().TempDir(), "missing"))

			Expect(err).To(MatchError(`reference "stand-in": licence is required`))
		})
	})
})

// treeListing is every path under root with its size, so a written or
// changed file shows.
func treeListing(root string) []string {
	GinkgoHelper()
	var listing []string
	Expect(filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		listing = append(listing, rel+" "+strings.Repeat("#", int(info.Size()%7)))
		return nil
	})).To(Succeed())
	return listing
}
