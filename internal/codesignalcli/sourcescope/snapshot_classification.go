package sourcescope

import (
	"os"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// classifySourceFiles labels each selected file's SourceScope without
// filtering any of them out, so Apply and
// ApplyBaseline can share the classification logic while
// applying different policies for what happens to test_only/excluded files.
func classifySourceFiles(dir, headSHA, buildTarget, scope string, files []gitrepo.SelectedFile) ([]gitrepo.SelectedFile, error) {
	if scope == "all" {
		classified := make([]gitrepo.SelectedFile, len(files))
		for i, file := range files {
			file.SourceScope = classifyFilename(file)
			classified[i] = file
		}
		return classified, nil
	}

	repositoryRoot, err := gitrepo.RepositoryRoot(dir)
	if err != nil {
		return nil, err
	}

	// Analysis reads only committed objects from headSHA. Build source scope
	// from the same snapshot so local edits cannot affect which findings
	// appear.
	snapshotDir, err := gitrepo.ExtractRevision(repositoryRoot, headSHA)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(snapshotDir)

	goProduction, err := goProductionFiles(snapshotDir, repositoryRoot, dir, buildTarget)
	if err != nil {
		return nil, err
	}
	config, hasTSConfig, err := loadTSConfig(snapshotDir)
	if err != nil {
		return nil, err
	}

	classified := make([]gitrepo.SelectedFile, len(files))
	for i, file := range files {
		file.SourceScope = classifySourceFile(file, goProduction, buildTarget, config, hasTSConfig)
		classified[i] = file
	}
	return classified, nil
}

func classifyFilename(file gitrepo.SelectedFile) string {
	if strings.HasSuffix(file.Path, "_test.go") {
		return TestOnly
	}
	return Unknown
}
