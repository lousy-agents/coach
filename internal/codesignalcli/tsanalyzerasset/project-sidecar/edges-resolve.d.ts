import type { Project } from "typescript/unstable/sync";
import type * as astns from "typescript/unstable/ast";
import type { ProjectSnapshot } from "./vfs.js";
export declare function resolveTarget(project: Project, specifierNode: astns.StringLiteral, specifierText: string, snapshot: ProjectSnapshot, fromVirtualFsPath: string, useChecker: boolean): {
    to: string;
    resolution: string;
};
//# sourceMappingURL=edges-resolve.d.ts.map