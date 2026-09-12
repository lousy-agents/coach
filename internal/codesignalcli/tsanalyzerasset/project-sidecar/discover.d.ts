import type { ProjectFile } from "./protocol.js";
export declare function discoverTsconfigPaths(files: readonly ProjectFile[], roots: readonly string[] | undefined): string[];
export declare function normalizeRoot(root: string): string;
/** An empty root or "." matches every path (an unscoped request treats the whole repository as its root); not exercised by any current fixture. */
export declare function isWithinRoot(path: string, root: string): boolean;
//# sourceMappingURL=discover.d.ts.map