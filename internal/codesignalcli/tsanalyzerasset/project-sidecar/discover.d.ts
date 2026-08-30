import type { ProjectFile } from "./protocol.js";
/**
 * Finds repository-relative tsconfig.json paths in the snapshot, scoped to
 * `roots` when non-empty (matching how pkg/projectmodel's Go client scopes
 * file collection to roots -- here it scopes which tsconfig(s) are treated
 * as project entry points, per Task 2's brief). Each discovered path is
 * opened as its own project (see analyze.ts); TypeScript's own project
 * reference following (proven in this package's design spikes) resolves
 * cross-project imports without this sidecar needing to parse `references`
 * arrays itself.
 */
export declare function discoverTsconfigPaths(files: readonly ProjectFile[], roots: readonly string[] | undefined): string[];
//# sourceMappingURL=discover.d.ts.map