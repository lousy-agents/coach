import { resolvePackageExports } from "./resolve-exports.js";
import { VIRTUAL_ROOT } from "./vfs.js";
/**
 * Manual, hand-rolled specifier resolver used only where the real TS
 * checker cannot help: require() calls (TS never tracks these without an
 * ambient `require` typing from @types/node, which snapshots do not ship)
 * and as a fallback when the checker's own resolution comes back empty.
 * Static import/export declarations are resolved primarily through the
 * real TS checker (see edges.ts).
 */
export function resolveSpecifier(fromVirtualPath, specifier, snapshot) {
    if (isRelativeSpecifier(specifier)) {
        const resolved = resolveRelative(fromVirtualPath, specifier, snapshot.nonMirrorVirtualPaths);
        return resolved ? { kind: "snapshot", virtualPath: resolved } : { kind: "unresolved" };
    }
    const resolved = resolvePackageSpecifier(specifier, snapshot);
    return resolved ? { kind: "snapshot", virtualPath: resolved } : { kind: "external" };
}
function isRelativeSpecifier(spec) {
    return spec === "." || spec === ".." || spec.startsWith("./") || spec.startsWith("../");
}
function resolveRelative(fromVirtualPath, specifier, paths) {
    const dir = fromVirtualPath.slice(0, fromVirtualPath.lastIndexOf("/"));
    const joined = normalizeVirtualPath(`${dir}/${specifier}`);
    return probeCandidates(joined, paths);
}
/**
 * Tries, in order: the exact path; a `.js`/`.jsx` specifier rewritten to
 * `.ts`/`.tsx`; `.ts`/`.tsx`/`.d.ts` appended; and `/index.ts`/`/index.tsx`.
 */
function probeCandidates(base, paths) {
    const rewritten = base.replace(/\.jsx$/, ".tsx").replace(/\.js$/, ".ts");
    const candidates = [
        rewritten,
        base,
        `${rewritten}.ts`,
        `${rewritten}.tsx`,
        `${rewritten}.d.ts`,
        `${rewritten}/index.ts`,
        `${rewritten}/index.tsx`,
    ];
    for (const candidate of candidates) {
        if (paths.has(candidate))
            return candidate;
    }
    return undefined;
}
function resolvePackageSpecifier(specifier, snapshot) {
    for (const info of snapshot.packagesByName.values()) {
        if (specifier !== info.name && !specifier.startsWith(`${info.name}/`))
            continue;
        const subpath = specifier === info.name ? "." : `.${specifier.slice(info.name.length)}`;
        const target = resolvePackageExports(info.exports, info.main, subpath);
        if (target === undefined)
            continue;
        const full = normalizeVirtualPath(`${info.dirVirtual}/${target}`);
        if (snapshot.nonMirrorVirtualPaths.has(full))
            return full;
        const probed = probeCandidates(full, snapshot.nonMirrorVirtualPaths);
        if (probed)
            return probed;
    }
    return undefined;
}
function normalizeVirtualPath(p) {
    const parts = p.split("/");
    const out = [];
    for (const part of parts) {
        if (part === "" || part === ".")
            continue;
        if (part === "..") {
            out.pop();
            continue;
        }
        out.push(part);
    }
    return out.length === 0 ? VIRTUAL_ROOT : `/${out.join("/")}`;
}
//# sourceMappingURL=resolve.js.map