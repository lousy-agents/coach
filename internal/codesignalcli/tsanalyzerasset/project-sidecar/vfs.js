import { createVirtualFileSystem } from "typescript/unstable/fs";
/**
 * All virtual paths live under this synthetic absolute root, disjoint from
 * any real filesystem path, so tsgo's absolute-path-based module
 * resolution always operates purely inside the request's snapshot.
 */
export const VIRTUAL_ROOT = "/coach-snapshot";
export function toVirtualPath(repoRelativePath) {
    const normalized = repoRelativePath.replace(/\\/g, "/").replace(/^\.\/+/, "").replace(/^\/+/, "");
    return `${VIRTUAL_ROOT}/${normalized}`;
}
/** Inverse of toVirtualPath; undefined for any path outside VIRTUAL_ROOT (e.g. tsgo's bundled lib.*.d.ts). */
export function fromVirtualPath(virtualPath) {
    const prefix = `${VIRTUAL_ROOT}/`;
    return virtualPath.startsWith(prefix) ? virtualPath.slice(prefix.length) : undefined;
}
/**
 * Builds the snapshot-confined virtual filesystem the sidecar hands to
 * `typescript/unstable/sync`'s API, plus a synthetic node_modules mirror
 * of every in-snapshot package.json'd directory.
 */
export function buildProjectSnapshot(files) {
    const inventory = indexSnapshotFiles(files);
    const packages = mirrorInSnapshotPackages(files, inventory.contentByPath);
    const record = { ...inventory.record, ...packages.mirrorRecord };
    return {
        fs: confinedFileSystem(record),
        originalVirtualPaths: inventory.originalVirtualPaths,
        packagesByName: packages.packagesByName,
        ...pathCanonicalizers(inventory.originalByLower, packages.mirrorToOriginal, packages.mirrorByLower),
    };
}
function indexSnapshotFiles(files) {
    const record = {};
    const originalVirtualPaths = new Set();
    const contentByPath = new Map();
    const originalByLower = new Map();
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
function mirrorInSnapshotPackages(files, contentByPath) {
    const packagesByName = new Map();
    const mirrorRecord = {};
    const mirrorToOriginal = new Map();
    const mirrorByLower = new Map();
    for (const f of files) {
        if (basename(f.path) !== "package.json")
            continue;
        const parsed = tryParseJson(contentByPath.get(f.path) ?? "");
        if (!isRecord(parsed) || typeof parsed.name !== "string" || parsed.name === "")
            continue;
        const name = parsed.name;
        const dirRepo = dirnameRepo(f.path);
        packagesByName.set(name, {
            name,
            dirVirtual: toVirtualPath(dirRepo),
            exports: parsed.exports,
            main: typeof parsed.main === "string" ? parsed.main : undefined,
        });
        const tree = packageMirrorEntries(name, dirRepo, files, contentByPath);
        Object.assign(mirrorRecord, tree.record);
        for (const [k, v] of tree.mirrorToOriginal)
            mirrorToOriginal.set(k, v);
        for (const [k, v] of tree.mirrorByLower)
            mirrorByLower.set(k, v);
    }
    return { packagesByName, mirrorRecord, mirrorToOriginal, mirrorByLower };
}
function packageMirrorEntries(name, dirRepo, files, contentByPath) {
    const record = {};
    const mirrorToOriginal = new Map();
    const mirrorByLower = new Map();
    const mirrorDir = `${VIRTUAL_ROOT}/node_modules/${name}`;
    const prefix = dirRepo === "" ? "" : `${dirRepo}/`;
    for (const other of files) {
        if (!fileBelongsToPackage(other.path, dirRepo, prefix))
            continue;
        const suffix = other.path.slice(prefix.length);
        const mirrorPath = suffix === "" ? mirrorDir : `${mirrorDir}/${suffix}`;
        const originalVirtual = toVirtualPath(other.path);
        record[mirrorPath] = contentByPath.get(other.path) ?? "";
        mirrorToOriginal.set(mirrorPath, originalVirtual);
        mirrorByLower.set(mirrorPath.toLowerCase(), originalVirtual);
    }
    return { record, mirrorToOriginal, mirrorByLower };
}
function fileBelongsToPackage(path, dirRepo, prefix) {
    if (dirRepo !== "" && !path.startsWith(prefix))
        return false;
    if (dirRepo === "" && path.startsWith("node_modules/"))
        return false;
    return true;
}
function confinedFileSystem(record) {
    const base = createVirtualFileSystem(record);
    return {
        ...base,
        readFile: (fileName) => {
            const result = base.readFile ? base.readFile(fileName) : undefined;
            // Explicit null (not undefined) on a miss — undefined falls through
            // to the real filesystem and breaks snapshot confinement.
            return result === undefined ? null : result;
        },
    };
}
function pathCanonicalizers(originalByLower, mirrorToOriginal, mirrorByLower) {
    const unmirror = (virtualPath) => {
        return mirrorToOriginal.get(virtualPath) ?? mirrorByLower.get(virtualPath.toLowerCase()) ?? virtualPath;
    };
    const canonicalizeVirtualPath = (virtualPath) => {
        const unmirrored = unmirror(virtualPath);
        return originalByLower.get(unmirrored.toLowerCase()) ?? unmirrored;
    };
    const toRepoPath = (virtualPath) => {
        return fromVirtualPath(canonicalizeVirtualPath(virtualPath));
    };
    return { unmirror, canonicalizeVirtualPath, toRepoPath };
}
function basename(p) {
    const idx = p.lastIndexOf("/");
    return idx === -1 ? p : p.slice(idx + 1);
}
function dirnameRepo(p) {
    const idx = p.lastIndexOf("/");
    return idx === -1 ? "" : p.slice(0, idx);
}
function tryParseJson(text) {
    try {
        return JSON.parse(text);
    }
    catch {
        return undefined;
    }
}
export function isRecord(v) {
    return typeof v === "object" && v !== null && !Array.isArray(v);
}
//# sourceMappingURL=vfs.js.map