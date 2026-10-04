import type * as astns from "typescript/unstable/ast";
import type { Project } from "typescript/unstable/sync";
import type { AstCompiler } from "./reachability-types.js";
import type { ProjectSnapshot } from "./vfs.js";
export type GapKind = "type_only" | "unresolved_external" | "dynamic_import";
export declare function gapDiagnosticInfo(gap: GapKind): {
    code: string;
    message: string;
};
export declare function classifyCalleeGap(call: astns.CallExpression, project: Project, snapshot: ProjectSnapshot, compiler: AstCompiler): GapKind | undefined;
//# sourceMappingURL=reachability-gaps.d.ts.map