export function discoverTsconfigPaths(files, roots) {
    const scopedRoots = roots && roots.length > 0 ? roots.map(normalizeRoot) : undefined;
    const found = new Set();
    for (const f of files) {
        if (basename(f.path) !== "tsconfig.json")
            continue;
        if (scopedRoots && !scopedRoots.some((root) => isWithinRoot(f.path, root)))
            continue;
        found.add(f.path);
    }
    return [...found].sort();
}
function basename(p) {
    const idx = p.lastIndexOf("/");
    return idx === -1 ? p : p.slice(idx + 1);
}
export function normalizeRoot(root) {
    return root.replace(/^\.\/+/, "").replace(/\/+$/, "");
}
/** An empty root or "." matches every path (an unscoped request treats the whole repository as its root); not exercised by any current fixture. */
export function isWithinRoot(path, root) {
    if (root === "" || root === ".")
        return true;
    return path === root || path.startsWith(`${root}/`);
}
//# sourceMappingURL=discover.js.map