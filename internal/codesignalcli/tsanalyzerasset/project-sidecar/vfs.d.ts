import type { FileSystem } from "typescript/unstable/fs";
import type { ProjectFile } from "./protocol.js";
type CreateVirtualFileSystem = typeof import("typescript/unstable/fs").createVirtualFileSystem;
/**
 * All virtual paths live under this synthetic absolute root, disjoint from
 * any real filesystem path, so tsgo's absolute-path-based module
 * resolution always operates purely inside the request's snapshot.
 */
export declare const VIRTUAL_ROOT = "/coach-snapshot";
export declare function toVirtualPath(repoRelativePath: string): string;
export declare function fromVirtualPath(virtualPath: string): string | undefined;
export interface PackageJsonInfo {
    name: string;
    dirVirtual: string;
    exports?: unknown;
    main?: string;
}
export interface ProjectSnapshot {
    fs: FileSystem;
    nonMirrorVirtualPaths: ReadonlySet<string>;
    packagesByName: ReadonlyMap<string, PackageJsonInfo>;
    unmirror(virtualPath: string): string;
    /**
     * Rewrites a virtual path (mirror or original, any casing) to the
     * exact-cased original snapshot virtual path when one exists. On
     * case-insensitive hosts, TS's checker lowercases declaration paths;
     * stable `file:` IDs must still match the request inventory byte-for-byte.
     */
    canonicalizeVirtualPath(virtualPath: string): string;
    toRepoPath(virtualPath: string): string | undefined;
}
export declare function buildProjectSnapshot(files: readonly ProjectFile[], createVirtualFileSystem: CreateVirtualFileSystem): ProjectSnapshot;
export declare function isRecord(v: unknown): v is Record<string, unknown>;
export {};
//# sourceMappingURL=vfs.d.ts.map