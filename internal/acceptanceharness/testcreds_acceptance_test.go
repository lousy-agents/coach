package acceptanceharness_test

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

var _ = Describe("generated test-credential helpers", func() {
	Context("GenerateRSAPrivateKeyPEM", func() {
		It("returns bytes that PEM-decode to a PKCS#1 RSA private key", func() {
			pemBytes := acceptanceharness.GenerateRSAPrivateKeyPEM(GinkgoTB())

			block, rest := pem.Decode(pemBytes)
			Expect(block).NotTo(BeNil())
			Expect(rest).To(BeEmpty())
			Expect(block.Type).To(Equal("RSA PRIVATE KEY"))

			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			Expect(err).NotTo(HaveOccurred())
			Expect(key).To(BeAssignableToTypeOf(&rsa.PrivateKey{}))
		})

		It("generates a fresh key on every call, never a hardcoded fixture", func() {
			first := acceptanceharness.GenerateRSAPrivateKeyPEM(GinkgoTB())
			second := acceptanceharness.GenerateRSAPrivateKeyPEM(GinkgoTB())

			Expect(first).NotTo(Equal(second))
		})
	})

	Context("GenerateFixtureToken", func() {
		It("returns a non-guessable string carrying the caller-given prefix", func() {
			token := acceptanceharness.GenerateFixtureToken(GinkgoTB(), "test-oauth-")

			Expect(token).To(HavePrefix("test-oauth-"))
			Expect(len(token)).To(BeNumerically(">", len("test-oauth-")))
		})

		It("generates a fresh value on every call, never a static placeholder", func() {
			first := acceptanceharness.GenerateFixtureToken(GinkgoTB(), "test-installation-")
			second := acceptanceharness.GenerateFixtureToken(GinkgoTB(), "test-installation-")

			Expect(first).NotTo(Equal(second))
		})
	})
})
