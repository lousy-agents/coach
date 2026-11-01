import type { Project } from "typescript/unstable/sync";
import type * as astns from "typescript/unstable/ast";
import type { ProjectSnapshot } from "./vfs.js";
/** A static module specifier: a string literal, or a template literal with no substitutions. */
export type SpecifierNode = astns.StringLiteral | astns.NoSubstitutionTemplateLiteral;
export declare function resolveTarget(project: Project, specifierNode: SpecifierNode, specifierText: string, snapshot: ProjectSnapshot, fromVirtualFsPath: string, useChecker: boolean): {
    to: string;
    resolution: string;
};
//# sourceMappingURL=edges-resolve.d.ts.map