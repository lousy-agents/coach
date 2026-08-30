import type { Project } from "typescript/unstable/sync";
import type { Diagnostic, ImportEdgeFact } from "./protocol.js";
import type { ProjectSnapshot } from "./vfs.js";
export interface EdgeExtractionResult {
    edges: ImportEdgeFact[];
    diagnostics: Diagnostic[];
    /** Canonical virtual paths newly walked by this call (caller merges into its visit set). */
    visitedPaths: string[];
}
/**
 * Walks every .ts/.tsx file owned by `project` that has not already been
 * visited by another project in this request, extracting one ImportEdgeFact
 * per import/re-export/require/dynamic-import site.
 */
export declare function extractEdgesForProject(project: Project, snapshot: ProjectSnapshot, alreadyVisited: ReadonlySet<string>): EdgeExtractionResult;
//# sourceMappingURL=edges.d.ts.map