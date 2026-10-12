package baseline_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

var _ = Describe("repo_baseline_scan job handler", func() {
	When("GitHub fetch fails with not-found/auth", func() {
		It("fails with a sentinel-mapped actionable error", func() {
			src := &fakeTreeSource{listErr: githubingest.ErrNotFound}
			h := baseline.NewScanHandler(baseline.ScanConfig{
				TreeSource: src,
				Gateway:    modelgateway.NewStubGateway(),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "missing",
				Ref:       "main",
			}), w)
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrNotFound)).To(BeTrue(),
				"not-found fetch must remain errors.Is-compatible with githubingest.ErrNotFound; got %v", err)
			Expect(err.Error()).NotTo(BeEmpty())
			Expect(src.listCalls).To(BeNumerically(">=", 1))

			srcAuth := &fakeTreeSource{listErr: githubingest.ErrAuth}
			hAuth := baseline.NewScanHandler(baseline.ScanConfig{
				TreeSource: srcAuth,
				Gateway:    modelgateway.NewStubGateway(),
			})
			_, err = hAuth(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "private",
			}), newCaptureWriter())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrAuth)).To(BeTrue(),
				"auth fetch must remain errors.Is-compatible with githubingest.ErrAuth; got %v", err)
		})
	})

	When("ListFiles succeeds but ReadFile fails mid-fetch", func() {
		It("fails with ErrNotFound and persists no findings", func() {
			src := &fakeTreeSource{
				resolvedSHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				entries:     []baseline.FileEntry{{Path: "main.go", Size: 20}},
				readErr:     githubingest.ErrNotFound,
			}
			h := baseline.NewScanHandler(baseline.ScanConfig{
				TreeSource: src,
				Gateway:    modelgateway.NewStubGateway(),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "widgets",
				Ref:       "main",
			}), w)
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrNotFound)).To(BeTrue(),
				"mid-fetch ReadFile not-found must remain errors.Is-compatible; got %v", err)
			Expect(src.listCalls).To(BeNumerically(">=", 1))
			Expect(src.readCalls).To(BeNumerically(">=", 1),
				"handler must attempt ReadFile after a successful ListFiles")
			Expect(w.findings).To(BeEmpty(), "mid-fetch failure must not persist findings")
		})

		It("fails with ErrAuth and persists no findings", func() {
			src := &fakeTreeSource{
				resolvedSHA: "cccccccccccccccccccccccccccccccccccccccc",
				entries:     []baseline.FileEntry{{Path: "main.go", Size: 20}},
				readErr:     githubingest.ErrAuth,
			}
			h := baseline.NewScanHandler(baseline.ScanConfig{
				TreeSource: src,
				Gateway:    modelgateway.NewStubGateway(),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "private",
			}), w)
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrAuth)).To(BeTrue(),
				"mid-fetch ReadFile auth failure must remain errors.Is-compatible; got %v", err)
			Expect(src.readCalls).To(BeNumerically(">=", 1))
			Expect(w.findings).To(BeEmpty())
		})
	})

	When("the handler uses GitHubTreeSource against fake GitHub Contents", func() {
		It("completes a baseline via real ListFiles/ReadFile/ResolveCommitSHA, not a tree double", func() {
			expectGitHubTreeSourceBaselineCompletes()
		})
	})

	When("a GitHub-backed tree source resolves the analyzed commit", func() {
		It("records Completion.CommitSHA as the resolved object SHA, not the branch name or HEAD", func() {
			const resolved = "0123456789abcdef0123456789abcdef01234567"
			src := &fakeTreeSource{
				resolvedSHA: resolved,
				entries: []baseline.FileEntry{
					{Path: "main.go", Size: 20},
				},
				contents: map[string][]byte{
					"main.go": []byte("package main\n\nfunc main() {}\n"),
				},
			}
			h := baseline.NewScanHandler(baseline.ScanConfig{
				TreeSource: src,
				Gateway:    modelgateway.NewStubGateway(),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "widgets",
				Ref:       "main",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())
			Expect(completion.CommitSHA).To(Equal(resolved),
				"commit_sha must be the resolved commit object SHA, not the branch ref")
			Expect(completion.CommitSHA).NotTo(Equal("main"))
			Expect(completion.CommitSHA).NotTo(Equal("HEAD"))
			Expect(src.resolveCalls).To(BeNumerically(">=", 1))
			Expect(src.lastListRef).To(Equal(resolved))
			Expect(src.lastReadRef).To(Equal(resolved))
		})

		It("resolves an empty ref to a commit object SHA rather than inventing HEAD", func() {
			const resolved = "fedcba9876543210fedcba9876543210fedcba98"
			src := &fakeTreeSource{
				resolvedSHA: resolved,
				entries:     []baseline.FileEntry{{Path: "a.go", Size: 10}},
				contents:    map[string][]byte{"a.go": []byte("package a\n")},
			}
			h := baseline.NewScanHandler(baseline.ScanConfig{
				TreeSource: src,
				Gateway:    modelgateway.NewStubGateway(),
			})
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "widgets",
			}), newCaptureWriter())
			Expect(err).NotTo(HaveOccurred())
			Expect(completion.CommitSHA).To(Equal(resolved))
			Expect(completion.CommitSHA).NotTo(Equal("HEAD"))
			Expect(src.lastResolveRef).To(Equal(""), "handler must pass empty ref through to ResolveCommitSHA")
		})
	})
})
