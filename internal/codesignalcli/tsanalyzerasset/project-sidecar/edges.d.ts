import type { Project } from "typescript/unstable/sync";
import type { Diagnostic, ImportEdgeFact } from "./protocol.js";
import type { ProjectSnapshot } from "./vfs.js";
type AstModule = typeof import("typescript/unstable/ast");
export interface EdgeExtractionResult {
    edges: ImportEdgeFact[];
    diagnostics: Diagnostic[];
    newlyVisitedPaths: string[];
}
export declare function extractEdgesForProject(project: Project, snapshot: ProjectSnapshot, alreadyVisited: ReadonlySet<string>, ast: AstModule): EdgeExtractionResult;
export {};
//# sourceMappingURL=edges.d.ts.map