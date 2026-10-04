import type * as astns from "typescript/unstable/ast";
import type { Project } from "typescript/unstable/sync";
import type { ReachabilityAccumulator } from "./reachability-accumulator.js";
import type { AstCompiler } from "./reachability-types.js";
import type { ProjectSnapshot } from "./vfs.js";
export declare function isRouteRegistrationCall(call: astns.CallExpression, project: Project, compiler: AstCompiler): boolean;
export declare function processRouteRegistration(call: astns.CallExpression, sf: astns.SourceFile, project: Project, snapshot: ProjectSnapshot, acc: ReachabilityAccumulator, compiler: AstCompiler): void;
//# sourceMappingURL=reachability-routes.d.ts.map