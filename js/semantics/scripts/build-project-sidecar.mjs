#!/usr/bin/env node
/**
 * Produces the directly-executable pinned sidecar binary
 * js/semantics/bin/coach-ts-project-sidecar from the already-compiled
 * dist/project-sidecar/ tree (see `npm run build`, a prerequisite this
 * script does not re-run itself in its default mode -- see --clean below).
 *
 * The compiled tree is copied verbatim (no rewriting of its relative
 * import specifiers -- that would be a hand-rolled bundler, which issue
 * #214 explicitly rules out) to bin/project-sidecar/, and
 * bin/coach-ts-project-sidecar is a small hand-written ESM shim -- not
 * tsc output -- that imports the real entry module by relative path, so
 * Task 1's Go client (which spawns opts.BinaryPath directly, not via a
 * shell or `node <script>`) has exactly one pinned, extension-less,
 * executable path to invoke.
 *
 * `tsc` never prunes its own outDir, so a module deleted/renamed under
 * src/project-sidecar/ survives in dist/project-sidecar/ across an
 * incremental build -- and this script's own vendor-copy step would then
 * faithfully re-vendor that stale file into bin/ too, since it only
 * cleans bin/project-sidecar/, not dist/project-sidecar/. Run with
 * `--clean` (see npm run build:project-sidecar) BEFORE `tsc` runs, so the
 * compiled tree itself starts empty and can never carry a stale artifact
 * forward into either dist/ or the vendored bin/ copy.
 */
import { chmodSync, cpSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const packageRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const sourceDir = join(packageRoot, "dist", "project-sidecar");
const binDir = join(packageRoot, "bin");
const vendoredDir = join(binDir, "project-sidecar");
const dest = join(binDir, "coach-ts-project-sidecar");

if (process.argv.includes("--clean")) {
  // Removes only the project-sidecar subtree, not all of dist/, so the
  // rest of the package's tsc output is left untouched.
  rmSync(sourceDir, { recursive: true, force: true });
  console.log(`cleaned ${sourceDir}`);
  process.exit(0);
}

mkdirSync(binDir, { recursive: true });
rmSync(vendoredDir, { recursive: true, force: true });
cpSync(sourceDir, vendoredDir, { recursive: true });

const shim = "#!/usr/bin/env node\nimport \"./project-sidecar/main.js\";\n";
writeFileSync(dest, shim, { mode: 0o755 });
chmodSync(dest, 0o755);
console.log(`built ${dest}`);
