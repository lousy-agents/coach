package main

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/internal/acceptanceharness/thinproof"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/githubingest"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func scanThinproof(session thinproofSession) error {
	ctx := context.Background()
	content, meta, err := session.reader.ReadFile(ctx, githubingest.GitHubFileRef{
		Owner: thinproof.Owner,
		Repo:  thinproof.Repo,
		Ref:   thinproof.Ref,
		Path:  thinproof.Path,
	})
	if err != nil {
		return fmt.Errorf("step 7 (ReadFile against fake GitHub): %w", err)
	}

	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		return fmt.Errorf("step 8 (NewAnalyzer): %w", err)
	}
	result, err := analyzer.AnalyzeBytes(ctx, semantics.FileInput{
		Path:     thinproof.Path,
		Language: semantics.LanguageGo,
		Content:  content,
	})
	if err != nil {
		return fmt.Errorf("step 8 (AnalyzeBytes): %w", err)
	}

	builder, err := codesignal.New(codesignal.Options{})
	if err != nil {
		return fmt.Errorf("step 9 (codesignal.New): %w", err)
	}
	report, err := builder.Build(ctx, codesignal.Input{
		Files: []codesignal.FileChange{
			{Path: thinproof.Path, Status: "modified", Head: result},
		},
	})
	if err != nil {
		return fmt.Errorf("step 9 (codesignal Build): %w", err)
	}

	records, err := fetchFakeGitHubRecords(ctx, session.client, session.baseURL)
	if err != nil {
		return fmt.Errorf("step 10 (fetch /__test__/records): %w", err)
	}

	out := thinproofResult{
		SchemaVersion:     resultSchemaVersion,
		Report:            report,
		FileMetadata:      meta,
		GuardResult:       session.guardResult,
		BlockedRequests:   session.transport.BlockedRequests(),
		FakeGitHubRecords: records,
	}
	if err := writeResult(session.outputPath, out); err != nil {
		return fmt.Errorf("step 11 (write OUTPUT_PATH): %w", err)
	}
	return nil
}
