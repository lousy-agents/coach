import type * as astns from "typescript/unstable/ast";
import type { ProjectSnapshot } from "./vfs.js";
/**
 * Walks .parent from a resolved method declaration to the nearest
 * enclosing class/interface name -- e.g. `prisma.user.findMany()`
 * resolves to a MethodDeclaration nested inside an object-literal
 * property of class PrismaClient, so the enclosing class (not the
 * immediate object-literal type) names the sink -- then defers to
 * sinkNodeId for both the name and module-provenance match.
 */
export declare function sinkIdForDeclaration(declNode: astns.Node, ast: typeof astns): string | undefined;
export declare function isUnfollowedLocalCallee(declNode: astns.Node, snapshot: ProjectSnapshot, ast: typeof astns): boolean;
//# sourceMappingURL=reachability-sink.d.ts.map