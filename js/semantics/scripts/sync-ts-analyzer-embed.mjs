#!/usr/bin/env node
/**
 * Syncs the compiled TypeScript project-sidecar build output
 * (js/semantics/bin/coach-ts-project-sidecar + bin/project-sidecar/) into
 * the checked-in embed directory internal/codesignalcli/tsanalyzerasset/,
 * which //go:embed reads at Go compile time.
 *
 * Run `npm run build:project-sidecar` first (see mise's project-sidecar-build
 * task, which this script's own mise tasks depend on) -- this script only
 * copies bin/, it does not build it.
 *
 * mise's ts-analyzer-embed-stale-check task re-runs this script and then
 * `git diff --exit-code`s the destination, mirroring tidy-check's
 * `go mod tidy && git diff --exit-code go.mod go.sum` pattern for this
 * embedded asset instead of go.mod/go.sum.
 */
import { chmodSync, cpSync, existsSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const packageRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const repoRoot = dirname(dirname(packageRoot));

const shimSrc = join(packageRoot, "bin", "coach-ts-project-sidecar");
const treeSrc = join(packageRoot, "bin", "project-sidecar");
const destDir = join(repoRoot, "internal", "codesignalcli", "tsanalyzerasset");
const shimDest = join(destDir, "coach-ts-project-sidecar");
const treeDest = join(destDir, "project-sidecar");

if (!existsSync(shimSrc) || !existsSync(treeSrc)) {
  console.error(
    `sync-ts-analyzer-embed: missing ${shimSrc} or ${treeSrc} -- run "npm run build:project-sidecar" first`,
  );
  process.exit(1);
}

rmSync(destDir, { recursive: true, force: true });
mkdirSync(destDir, { recursive: true });
cpSync(shimSrc, shimDest);
chmodSync(shimDest, 0o755);
cpSync(treeSrc, treeDest, { recursive: true });
writeFileSync(join(destDir, "package.json"), `${JSON.stringify({ type: "module" })}\n`);

console.log(`synced ${destDir}`);
