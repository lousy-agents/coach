package githubingest_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// ExampleNewGitHubFileReader shows constructing a GitHubFileReader from a
// GitHub App's credentials. Construction never touches the network -
// authentication happens lazily on the first ReadFile call. The private key
// below is generated on the spot purely so this example runs offline and
// deterministically; a real caller would load their GitHub App's issued PEM
// key instead.
func ExampleNewGitHubFileReader() {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println(err)
		return
	}
	pemKey := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          123,
		InstallationID: 456,
		PrivateKey:     pemKey,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(reader != nil)
	// Output:
	// true
}
