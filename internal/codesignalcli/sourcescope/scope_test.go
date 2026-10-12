package sourcescope

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeTalliesExcludedFiles(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"shipping/shipping.go":      "package shipping\n\nfunc Update() {}\n",
		"shipping/shipping_test.go": "package shipping\n\nfunc TestUpdate() {}\n",
	})
	head := scopeTestCommit(t, repo)

	kept, excluded, err := Apply(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "shipping/shipping.go", Status: "modified", Language: semantics.LanguageGo},
		{Path: "shipping/shipping_test.go", Status: "modified", Language: semantics.LanguageGo},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}

	if len(kept) != 1 || kept[0].Path != "shipping/shipping.go" || kept[0].SourceScope != Unknown {
		t.Fatalf("ApplySourceScope() kept = %#v, want only shipping.go", kept)
	}

	if len(excluded) != 1 || excluded[0].Reason != TestOnly || excluded[0].Language != string(semantics.LanguageGo) || excluded[0].Count != 1 {
		t.Fatalf("ApplySourceScope() excluded = %#v, want one test_only/go group of count 1", excluded)
	}
}

func TestApplyBaselineSourceScopeTalliesExcludedFiles(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"shipping/shipping.go":      "package shipping\n\nfunc Update() {}\n",
		"shipping/shipping_test.go": "package shipping\n\nfunc TestUpdate() {}\n",
	})
	head := scopeTestCommit(t, repo)

	kept, excluded, err := ApplyBaseline(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "shipping/shipping.go", Language: semantics.LanguageGo},
		{Path: "shipping/shipping_test.go", Language: semantics.LanguageGo},
	})
	if err != nil {
		t.Fatalf("ApplyBaselineSourceScope() error = %v", err)
	}

	if len(kept) != 1 || kept[0].Path != "shipping/shipping.go" || kept[0].SourceScope != Unknown {
		t.Fatalf("ApplyBaselineSourceScope() kept = %#v, want only shipping.go", kept)
	}

	if len(excluded) != 1 || excluded[0].Reason != TestOnly || excluded[0].Language != string(semantics.LanguageGo) || excluded[0].Count != 1 {
		t.Fatalf("ApplyBaselineSourceScope() excluded = %#v, want one test_only/go group of count 1", excluded)
	}
}

func TestApplyBaselineSourceScopeAllReturnsEverythingUnexcluded(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"shipping/shipping.go":      "package shipping\n\nfunc Update() {}\n",
		"shipping/shipping_test.go": "package shipping\n\nfunc TestUpdate() {}\n",
	})
	head := scopeTestCommit(t, repo)

	kept, excluded, err := ApplyBaseline(repo, head, "", "all", []gitrepo.SelectedFile{
		{Path: "shipping/shipping.go", Language: semantics.LanguageGo},
		{Path: "shipping/shipping_test.go", Language: semantics.LanguageGo},
	})
	if err != nil {
		t.Fatalf("ApplyBaselineSourceScope() error = %v", err)
	}

	if len(kept) != 2 {
		t.Fatalf("ApplyBaselineSourceScope() kept = %#v, want both files for scope=all", kept)
	}
	if len(excluded) != 0 {
		t.Fatalf("ApplyBaselineSourceScope() excluded = %#v, want empty for scope=all", excluded)
	}
}
