package baseline_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/fakegithub"
)

// newGitHubBaselineTreeFixture builds a fakegithub Contents + commit graph that
// GitHubBaselineTreeSource can walk end-to-end (resolve → list → read).
// files keys must be top-level basenames (no nested paths).
func newGitHubBaselineTreeFixture(objectSHA string, files map[string][]byte) *fakegithub.Fixture {
	GinkgoHelper()
	fx := fakegithub.NewFixture("handler-github-tree-fixture")
	fx.Installation.Installations[42] = fakegithub.InstallationEntry{
		Token: "handler-install-token", Scenario: fakegithub.ScenarioOK,
	}
	fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{
		InstallationID: 42, Scenario: fakegithub.ScenarioOK,
	}
	fx.Repos.Repos["acme/widgets"] = fakegithub.RepoMetaEntry{
		DefaultBranch: "main", Scenario: fakegithub.ScenarioOK,
	}
	fx.Repos.Commits["acme/widgets/main"] = fakegithub.CommitEntry{
		SHA: objectSHA, Scenario: fakegithub.ScenarioOK,
	}
	fx.Repos.Commits["acme/widgets/"+objectSHA] = fakegithub.CommitEntry{
		SHA: objectSHA, Scenario: fakegithub.ScenarioOK,
	}

	rootKey := "acme/widgets/" + objectSHA
	rootEntries := make([]fakegithub.DirEntry, 0, len(files))
	i := 0
	for path, body := range files {
		Expect(path).NotTo(ContainSubstring("/"), "fixture helper supports top-level paths only")
		i++
		blob := fmt.Sprintf("blob%d", i)
		fx.Contents.Files[rootKey+"/"+path] = fakegithub.FileEntry{
			Content: body, SHA: blob, Scenario: fakegithub.ScenarioOK,
		}
		rootEntries = append(rootEntries, fakegithub.DirEntry{
			Name: path, Type: "file", SHA: blob, Size: len(body),
		})
	}
	// Parent listing for top-level ReadFile symlink checks.
	fx.Contents.Dirs[rootKey] = rootEntries
	return &fx
}

func baselineRSAKey() []byte {
	GinkgoHelper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block)
}
