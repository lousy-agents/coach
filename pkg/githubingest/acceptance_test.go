package githubingest_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// ginkgoRSAKey returns a freshly generated RSA private key, PKCS#1-PEM-
// encoded the same way GitHub encodes App private keys it issues. It never
// touches the network or any real credentials. Ginkgo-local variant of
// testhelpers_test.go's generateTestRSAPrivateKeyPEM, which takes a concrete
// *testing.T that a Ginkgo spec (running under GinkgoT(), a testing.TB) does
// not have.
func ginkgoRSAKey() []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block)
}

// ginkgoTestReader builds a GitHubFileReader wired to an offline fake
// transport (the ghinstallation token mint is answered automatically;
// handleContents answers the Contents API call under test). Ginkgo-local
// variant of testhelpers_test.go's newTestReader, for the same reason as
// ginkgoRSAKey above.
func ginkgoTestReader(handleContents contentsHandlerFunc) *githubingest.GitHubFileReader {
	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          12345,
		InstallationID: 67890,
		PrivateKey:     ginkgoRSAKey(),
		Transport:      &fakeGitHubTransport{handleContents: handleContents},
	})
	Expect(err).NotTo(HaveOccurred())
	return reader
}

const cannedFileResponse = `{
	"type": "file",
	"encoding": "base64",
	"size": 11,
	"name": "hello.txt",
	"path": "dir/hello.txt",
	"sha": "abc123sha",
	"content": "aGVsbG8gd29ybGQ="
}`

var _ = Describe("optional GitHub App file ingestion", func() {
	Context("when a Coach maintainer configures a GitHubFileReader", func() {
		It("builds an authenticated reader from a complete GitHubAppConfig, entirely offline (AC-5.2)", func() {
			cfg := githubingest.GitHubAppConfig{
				AppID:          12345,
				InstallationID: 67890,
				PrivateKey:     ginkgoRSAKey(),
			}

			reader, err := githubingest.NewGitHubFileReader(cfg)

			Expect(err).NotTo(HaveOccurred())
			Expect(reader).NotTo(BeNil())
		})

		It("targets a configured GitHub Enterprise BaseURL instead of github.com (AC-5.3)", func() {
			transport := &urlRecordingEnterpriseTransport{}
			reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
				AppID:          1,
				InstallationID: 2,
				PrivateKey:     ginkgoRSAKey(),
				BaseURL:        "https://ghe.example.com/",
				Transport:      transport,
			})
			Expect(err).NotTo(HaveOccurred())

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "hello.txt"}
			_, _, err = reader.ReadFile(context.Background(), ref)
			Expect(err).NotTo(HaveOccurred())

			for _, u := range transport.seen {
				Expect(u).To(HavePrefix("https://ghe.example.com/api/v3/"))
			}
		})
	})

	Context("when ReadFile succeeds", func() {
		It("returns the decoded raw bytes and file metadata (AC-5.4)", func() {
			reader := ginkgoTestReader(func(req *http.Request) *http.Response {
				return jsonResponse(req, http.StatusOK, cannedFileResponse)
			})

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/hello.txt"}
			data, meta, err := reader.ReadFile(context.Background(), ref)

			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("hello world"))
			Expect(meta).To(Equal(githubingest.FileMetadata{Path: "dir/hello.txt", Ref: "main", SHA: "abc123sha", Size: 11}))
		})
	})

	Context("when the GitHub Contents API rejects the request", func() {
		DescribeTable("classifies the response into the documented sentinel error",
			func(status int, wantErr error) {
				reader := ginkgoTestReader(func(req *http.Request) *http.Response {
					return jsonResponse(req, status, `{"message":"denied"}`)
				})

				ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "secret.txt"}
				_, _, err := reader.ReadFile(context.Background(), ref)

				Expect(errors.Is(err, wantErr)).To(BeTrue(), "got err %v, want errors.Is(err, %v)", err, wantErr)
			},
			Entry("404 Not Found (AC-5.5)", http.StatusNotFound, githubingest.ErrNotFound),
			Entry("401 Unauthorized (AC-5.6)", http.StatusUnauthorized, githubingest.ErrAuth),
			Entry("403 Forbidden (AC-5.6)", http.StatusForbidden, githubingest.ErrAuth),
		)
	})

	Context("when the requested path is not a regular file (AC-5.7)", func() {
		DescribeTable("returns an error matching ErrUnsupportedContent",
			func(body string) {
				reader := ginkgoTestReader(func(req *http.Request) *http.Response {
					return jsonResponse(req, http.StatusOK, body)
				})

				ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir"}
				_, _, err := reader.ReadFile(context.Background(), ref)

				Expect(errors.Is(err, githubingest.ErrUnsupportedContent)).To(BeTrue())
			},
			Entry("a directory listing", `[
				{"type":"file","name":"a.txt","path":"dir/a.txt","sha":"a","size":1},
				{"type":"file","name":"b.txt","path":"dir/b.txt","sha":"b","size":1}
			]`),
			Entry("a symlink", `{"type":"symlink","name":"link","path":"dir/link","sha":"sha1","size":9,"target":"../elsewhere"}`),
			Entry("a submodule", `{"type":"submodule","name":"vendor/lib","path":"vendor/lib","sha":"sha2","size":0,`+
				`"submodule_git_url":"git://example.com/lib.git"}`),
		)
	})

	Context("when the file exceeds the Contents API's size limit (AC-5.8)", func() {
		It("returns ErrTooLarge and no bytes, without attempting to decode the (unusable) content", func() {
			const canned = `{
				"type": "file",
				"encoding": "none",
				"size": 1048577,
				"name": "big.bin",
				"path": "dir/big.bin",
				"sha": "bigsha",
				"content": "not-valid-base64-and-must-never-be-decoded"
			}`
			reader := ginkgoTestReader(func(req *http.Request) *http.Response {
				return jsonResponse(req, http.StatusOK, canned)
			})

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/big.bin"}
			data, _, err := reader.ReadFile(context.Background(), ref)

			Expect(errors.Is(err, githubingest.ErrTooLarge)).To(BeTrue())
			Expect(data).To(BeNil())
		})
	})

	Context("when the decoded file content is empty (AC-5.9)", func() {
		It("returns ErrEmptyContent", func() {
			const canned = `{
				"type": "file",
				"encoding": "base64",
				"size": 0,
				"name": "empty.txt",
				"path": "dir/empty.txt",
				"sha": "emptysha",
				"content": ""
			}`
			reader := ginkgoTestReader(func(req *http.Request) *http.Response {
				return jsonResponse(req, http.StatusOK, canned)
			})

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/empty.txt"}
			_, _, err := reader.ReadFile(context.Background(), ref)

			Expect(errors.Is(err, githubingest.ErrEmptyContent)).To(BeTrue())
		})
	})

	Context("when the returned content fails to base64-decode (AC-5.11)", func() {
		It("returns a wrapped API-failure error matching none of the defined sentinels", func() {
			const canned = `{
				"type": "file",
				"encoding": "base64",
				"size": 5,
				"name": "bad.txt",
				"path": "dir/bad.txt",
				"sha": "badsha",
				"content": "!!!not-valid-base64!!!"
			}`
			reader := ginkgoTestReader(func(req *http.Request) *http.Response {
				return jsonResponse(req, http.StatusOK, canned)
			})

			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "dir/bad.txt"}
			_, _, err := reader.ReadFile(context.Background(), ref)

			Expect(err).To(HaveOccurred())
			for _, sentinel := range []error{
				githubingest.ErrAuth,
				githubingest.ErrNotFound,
				githubingest.ErrUnsupportedContent,
				githubingest.ErrEmptyContent,
				githubingest.ErrTooLarge,
			} {
				Expect(errors.Is(err, sentinel)).To(BeFalse(), "err %v unexpectedly matched sentinel %v", err, sentinel)
			}
		})
	})
})
