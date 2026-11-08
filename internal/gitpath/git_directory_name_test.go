package gitpath

import "testing"

func TestNamesGitDirectoryFollowsTheSpellingsGitRefusesToCheckOut(t *testing.T) {
	refused := []string{
		".git", ".GIT", ".gIt", "git~1", "GIT~1", "Git~1",
		".git.", ".git ", ".git. .", ".git...", "git~1.", "git~1 ",
		".git:stream", ".git::$INDEX_ALLOCATION", ".git\\config", "git~1\\x",
		".g\u200cit", ".\ufeffgit", "\u200c.git", ".git\u200f", ".gi\u202at", ".gi\u206ft", "\u202e.git",
	}
	for _, name := range refused {
		if !NamesGitDirectory(name) {
			t.Errorf("NamesGitDirectory(%q) = false; git refuses to check out a path with this component", name)
		}
	}
	kept := []string{
		".gitignore", ".github", ".gitattributes", ".gitmodules", ".git-keep", ".gitx", ".gi", ".g", "git", "gitx", "agit", "a.git", "..git",
		"git~2", "git~12", "git~", "git~1x", ".git x", ".git.x", "main.go", "", ".", "..",
	}
	for _, name := range kept {
		if NamesGitDirectory(name) {
			t.Errorf("NamesGitDirectory(%q) = true; git checks out a path with this component", name)
		}
	}
}

func TestRefuseUncheckablePathNamesTheComponentThatIsNotAName(t *testing.T) {
	refused := []string{"a/.git/config", ".git", "x/y/.GIT", "dir/git~1/HEAD", "a//b", "a/./b", "a/../b", "a/b/", "/a", ".", ".."}
	for _, path := range refused {
		if err := RefuseUncheckable(path); err == nil {
			t.Errorf("RefuseUncheckable(%q) = nil; want the path refused", path)
		}
	}
	kept := []string{"a.go", "dir/sub/main.go", ".github/workflows/ci.yml", "a/.gitignore", "src/git/x.go", "pkg/.gitkeep"}
	for _, path := range kept {
		if err := RefuseUncheckable(path); err != nil {
			t.Errorf("RefuseUncheckable(%q) = %v; want the path kept", path, err)
		}
	}
}
