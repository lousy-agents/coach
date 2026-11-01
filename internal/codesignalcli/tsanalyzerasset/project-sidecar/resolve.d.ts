import { type ProjectSnapshot } from "./vfs.js";
export type ResolvedKind = "snapshot" | "external" | "unresolved";
export interface ResolvedSpecifier {
    kind: ResolvedKind;
    /** Set only when kind === "snapshot". */
    virtualPath?: string;
}
/**
 * tsconfig `paths` as the checker sees them: the pattern table plus the
 * virtual directory its targets are relative to (the declaring config's
 * directory, which differs from the project's own when `paths` is inherited
 * through `extends`).
 */
export interface PathAliases {
    paths: Readonly<Record<string, readonly string[]>>;
    baseDirVirtual: string;
}
/**
 * Manual, hand-rolled specifier resolver used only where the real TS
 * checker cannot help: require() calls (TS never tracks these without an
 * ambient `require` typing from @types/node, which snapshots do not ship)
 * and as a fallback when the checker's own resolution comes back empty.
 * Static import/export declarations are resolved primarily through the
 * real TS checker (see edges.ts).
 *
 * A bare specifier is tried against `aliases` first, then against workspace
 * packages. A specifier that an alias pattern claims but whose targets are
 * missing is "unresolved" rather than "external": the snapshot has no
 * node_modules, so the only way to tell a broken alias from a package is that
 * a package is never claimed by a specific pattern. A catch-all `*` pattern
 * claims every package, so its misses stay "external".
 */
export declare function resolveSpecifier(fromVirtualPath: string, specifier: string, snapshot: ProjectSnapshot, aliases?: PathAliases): ResolvedSpecifier;
//# sourceMappingURL=resolve.d.ts.map