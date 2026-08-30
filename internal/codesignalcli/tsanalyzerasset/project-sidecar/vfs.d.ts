import { type FileSystem } from "typescript/unstable/fs";
import type { ProjectFile } from "./protocol.js";
/**
 * All virtual paths live under this synthetic absolute root, disjoint from
 * any real filesystem path, so tsgo's absolute-path-based module
 * resolution always operates purely inside the request's snapshot.
 */
export declare const VIRTUAL_ROOT = "/coach-snapshot";
export declare function toVirtualPath(repoRelativePath: string): string;
/** Inverse of toVirtualPath; undefined for any path outside VIRTUAL_ROOT (e.g. tsgo's bundled lib.*.d.ts). */
export declare function fromVirtualPath(virtualPath: string): string | undefined;
/** Parsed subset of a snapshot package.json used for bare-specifier resolution. */
export interface PackageJsonInfo {
    name: string;
    /** Virtual directory containing this package.json. */
    dirVirtual: string;
    exports?: unknown;
    main?: string;
}
export interface ProjectSnapshot {
    fs: FileSystem;
    /** Every virtual path present in the ORIGINAL snapshot (excludes the
     * synthetic node_modules mirror below), used by the manual fallback
     * resolver in resolve.ts. */
    originalVirtualPaths: ReadonlySet<string>;
    /** Snapshot package.json files keyed by their declared "name", used to
     * resolve bare specifiers that reference an in-snapshot package. */
    packagesByName: ReadonlyMap<string, PackageJsonInfo>;
    /** Maps a synthetic node_modules mirror path back to the original
     * snapshot virtual path it shadows, so an edge resolved through the
     * mirror is reported against the real repository path. */
    unmirror(virtualPath: string): string;
    /**
     * Rewrites a virtual path (mirror or original, any casing) to the
     * exact-cased original snapshot virtual path when one exists. On
     * case-insensitive hosts, TS's checker lowercases declaration paths;
     * stable `file:` IDs must still match the request inventory byte-for-byte.
     */
    canonicalizeVirtualPath(virtualPath: string): string;
    /**
     * Like fromVirtualPath, but after unmirror + case canonicalization so
     * the returned repo-relative path matches a request inventory entry
     * exactly when the virtual path names a snapshot file.
     */
    toRepoPath(virtualPath: string): string | undefined;
}
/**
 * Builds the snapshot-confined virtual filesystem the sidecar hands to
 * `typescript/unstable/sync`'s API, plus a synthetic node_modules mirror
 * of every in-snapshot package.json'd directory.
 */
export declare function buildProjectSnapshot(files: readonly ProjectFile[]): ProjectSnapshot;
export declare function isRecord(v: unknown): v is Record<string, unknown>;
//# sourceMappingURL=vfs.d.ts.map