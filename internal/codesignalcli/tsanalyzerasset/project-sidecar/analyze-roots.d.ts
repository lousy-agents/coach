import type { Project } from "typescript/unstable/sync";
import type { RootScopeFact } from "./protocol.js";
import { type ProjectSnapshot } from "./vfs.js";
export declare function computeRootScopes(roots: readonly string[] | undefined, projects: readonly Project[], snapshot: ProjectSnapshot, visited: ReadonlySet<string>): RootScopeFact[] | undefined;
//# sourceMappingURL=analyze-roots.d.ts.map