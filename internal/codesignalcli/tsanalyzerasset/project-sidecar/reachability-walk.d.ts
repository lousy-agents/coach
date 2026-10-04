import type * as astns from "typescript/unstable/ast";
import type { Project } from "typescript/unstable/sync";
import type { ReachabilityAccumulator } from "./reachability-accumulator.js";
import type { AstCompiler } from "./reachability-types.js";
import type { ProjectSnapshot } from "./vfs.js";
export declare function walkSourceForReachability(fn: astns.FunctionDeclaration, sourceId: string, acc: ReachabilityAccumulator, snapshot: ProjectSnapshot, project: Project, compiler: AstCompiler): void;
//# sourceMappingURL=reachability-walk.d.ts.map