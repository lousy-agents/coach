import type * as astns from "typescript/unstable/ast";
import type { Project } from "typescript/unstable/sync";
import { type ImportEdgeFact } from "./protocol.js";
import type { ProjectSnapshot } from "./vfs.js";
export declare function collectEdgesFromSourceFile(sf: astns.SourceFile, repoPath: string, project: Project, snapshot: ProjectSnapshot, ast: typeof astns): ImportEdgeFact[];
//# sourceMappingURL=edges-walk.d.ts.map