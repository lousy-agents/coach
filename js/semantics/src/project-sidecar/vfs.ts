import { createVirtualFileSystem, type FileSystem } from "typescript/unstable/fs";

import type { ProjectFile } from "./protocol.js";

/**
 * All virtual paths live under this synthetic absolute root, disjoint from
 * any real filesystem path, so tsgo's absolute-path-based module
 * resolution always operates purely inside the request's snapshot.
 */
export const VIRTUAL_ROOT = "/coach-snapshot";

export function toVirtualPath(repoRelativePath: string): string {
  const normalized = repoRelativePath.replace(/\\/g, "/").replace(/^\.\/+/, "").replace(/^\/+/, "");
  return `${VIRTUAL_ROOT}/${normalized}`;
}

/** Inverse of toVirtualPath; undefined for any path outside VIRTUAL_ROOT (e.g. tsgo's bundled lib.*.d.ts). */
export function fromVirtualPath(virtualPath: string): string | undefined {
  const prefix = `${VIRTUAL_ROOT}/`;
  return virtualPath.startsWith(prefix) ? virtualPath.slice(prefix.length) : undefined;
}

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
 * of every in-snapshot package.json'd directory. TS's own module
 * resolution only discovers package.json "exports"/"main" fields for
 * packages it finds by walking up node_modules directories from the
 * importing file -- mirroring each snapshot package under
 * `node_modules/<name>` lets the real (already-correct) TS resolver
 * satisfy issue #214's requirement 4 natively, instead of this sidecar
 * hand-rolling Node's exports algorithm for the primary import/export
 * case. (A narrower hand-rolled fallback still exists in resolve.ts, for
 * require()/dynamic-import cases the checker does not resolve; see there.)
 */
export function buildProjectSnapshot(files: readonly ProjectFile[]): ProjectSnapshot {
  const inventory = indexSnapshotFiles(files);
  const packages = mirrorInSnapshotPackages(files, inventory);

  return {
    fs: confinedFileSystem(inventory.record),
    originalVirtualPaths: inventory.originalVirtualPaths,
    packagesByName: packages.packagesByName,
    ...pathCanonicalizers(inventory.originalByLower, packages.mirrorToOriginal, packages.mirrorByLower),
  };
}

interface SnapshotInventory {
  record: Record<string, string>;
  originalVirtualPaths: Set<string>;
  contentByPath: Map<string, string>;
  /** Lowercased virtual path -> exact-cased original snapshot virtual path. */
  originalByLower: Map<string, string>;
}

function indexSnapshotFiles(files: readonly ProjectFile[]): SnapshotInventory {
  const record: Record<string, string> = {};
  const originalVirtualPaths = new Set<string>();
  const contentByPath = new Map<string, string>();
  const originalByLower = new Map<string, string>();

  for (const f of files) {
    const content = Buffer.from(f.content_b64, "base64").toString("utf8");
    const vpath = toVirtualPath(f.path);
    record[vpath] = content;
    originalVirtualPaths.add(vpath);
    originalByLower.set(vpath.toLowerCase(), vpath);
    contentByPath.set(f.path, content);
  }

  return { record, originalVirtualPaths, contentByPath, originalByLower };
}

interface PackageMirrorTables {
  packagesByName: Map<string, PackageJsonInfo>;
  mirrorToOriginal: Map<string, string>;
  mirrorByLower: Map<string, string>;
}

function mirrorInSnapshotPackages(files: readonly ProjectFile[], inventory: SnapshotInventory): PackageMirrorTables {
  const packagesByName = new Map<string, PackageJsonInfo>();
  const mirrorToOriginal = new Map<string, string>();
  const mirrorByLower = new Map<string, string>();

  for (const f of files) {
    if (basename(f.path) !== "package.json") continue;
    const parsed = tryParseJson(inventory.contentByPath.get(f.path) ?? "");
    if (!isRecord(parsed) || typeof parsed.name !== "string" || parsed.name === "") continue;

    const name = parsed.name;
    const dirRepo = dirnameRepo(f.path);
    packagesByName.set(name, {
      name,
      dirVirtual: toVirtualPath(dirRepo),
      exports: parsed.exports,
      main: typeof parsed.main === "string" ? parsed.main : undefined,
    });

    mirrorPackageTree(name, dirRepo, files, inventory, mirrorToOriginal, mirrorByLower);
  }

  return { packagesByName, mirrorToOriginal, mirrorByLower };
}

function mirrorPackageTree(
  name: string,
  dirRepo: string,
  files: readonly ProjectFile[],
  inventory: SnapshotInventory,
  mirrorToOriginal: Map<string, string>,
  mirrorByLower: Map<string, string>,
): void {
  const mirrorDir = `${VIRTUAL_ROOT}/node_modules/${name}`;
  const prefix = dirRepo === "" ? "" : `${dirRepo}/`;
  for (const other of files) {
    if (!fileBelongsToPackage(other.path, dirRepo, prefix)) continue;
    const suffix = other.path.slice(prefix.length);
    const mirrorPath = suffix === "" ? mirrorDir : `${mirrorDir}/${suffix}`;
    const originalVirtual = toVirtualPath(other.path);
    inventory.record[mirrorPath] = inventory.contentByPath.get(other.path) ?? "";
    mirrorToOriginal.set(mirrorPath, originalVirtual);
    mirrorByLower.set(mirrorPath.toLowerCase(), originalVirtual);
  }
}

function fileBelongsToPackage(path: string, dirRepo: string, prefix: string): boolean {
  if (dirRepo !== "" && !path.startsWith(prefix)) return false;
  if (dirRepo === "" && path.startsWith("node_modules/")) return false;
  return true;
}

function confinedFileSystem(record: Record<string, string>): FileSystem {
  const base = createVirtualFileSystem(record);
  return {
    ...base,
    readFile: (fileName) => {
      const result = base.readFile ? base.readFile(fileName) : undefined;
      // Explicit null (not undefined) on a miss. The FileSystem contract
      // documented on typescript/unstable/fs treats `undefined` as "fall
      // back to the real filesystem" -- returning it here would silently
      // break snapshot confinement, issue #214's core requirement.
      return result === undefined ? null : result;
    },
  };
}

function pathCanonicalizers(
  originalByLower: ReadonlyMap<string, string>,
  mirrorToOriginal: ReadonlyMap<string, string>,
  mirrorByLower: ReadonlyMap<string, string>,
): Pick<ProjectSnapshot, "unmirror" | "canonicalizeVirtualPath" | "toRepoPath"> {
  const unmirror = (virtualPath: string): string => {
    return mirrorToOriginal.get(virtualPath) ?? mirrorByLower.get(virtualPath.toLowerCase()) ?? virtualPath;
  };

  const canonicalizeVirtualPath = (virtualPath: string): string => {
    const unmirrored = unmirror(virtualPath);
    return originalByLower.get(unmirrored.toLowerCase()) ?? unmirrored;
  };

  const toRepoPath = (virtualPath: string): string | undefined => {
    return fromVirtualPath(canonicalizeVirtualPath(virtualPath));
  };

  return { unmirror, canonicalizeVirtualPath, toRepoPath };
}

function basename(p: string): string {
  const idx = p.lastIndexOf("/");
  return idx === -1 ? p : p.slice(idx + 1);
}

function dirnameRepo(p: string): string {
  const idx = p.lastIndexOf("/");
  return idx === -1 ? "" : p.slice(0, idx);
}

function tryParseJson(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return undefined;
  }
}

export function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}
