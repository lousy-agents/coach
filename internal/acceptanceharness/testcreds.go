package acceptanceharness

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"testing"
)

// GenerateRSAPrivateKeyPEM generates a fresh 2048-bit RSA private key and
// returns it PKCS#1-PEM-encoded -- the same shape GitHub uses for App
// private keys it issues. It never touches the network or any real
// credentials: the key is generated locally and discarded once the test
// using it completes. Any keygen or encoding failure fails tb immediately
// via tb.Fatal, so callers don't need their own error handling.
func GenerateRSAPrivateKeyPEM(tb testing.TB) []byte {
	tb.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		tb.Fatal(err)
	}

	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block)
}

// GenerateFixtureToken returns a random, non-guessable string suitable for
// use as a generated fixture OAuth/installation/access token in offline test
// fixtures. It is built entirely from crypto/rand output, so it never
// resembles or embeds a real credential, and it carries the caller-given
// prefix (e.g. "test-oauth-", "test-installation-") so it's obvious in logs
// and golden files that the value is synthetic. Any randomness failure fails
// tb immediately via tb.Fatal.
func GenerateFixtureToken(tb testing.TB, prefix string) string {
	tb.Helper()

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		tb.Fatal(err)
	}

	return prefix + hex.EncodeToString(buf)
}
