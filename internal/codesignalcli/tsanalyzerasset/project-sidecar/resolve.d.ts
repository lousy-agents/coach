import { type ProjectSnapshot } from "./vfs.js";
export type ResolvedKind = "snapshot" | "external" | "unresolved";
export interface ResolvedSpecifier {
    kind: ResolvedKind;
    /** Set only when kind === "snapshot". */
    virtualPath?: string;
}
/**
 * Manual, hand-rolled specifier resolver used only where the real TS
 * checker cannot help: require() calls (TS never tracks these without an
 * ambient `require` typing from @types/node, which snapshots do not ship)
 * and as a fallback when the checker's own resolution comes back empty.
 * Static import/export declarations are resolved primarily through the
 * real TS checker (see edges.ts).
 */
export declare function resolveSpecifier(fromVirtualPath: string, specifier: string, snapshot: ProjectSnapshot): ResolvedSpecifier;
//# sourceMappingURL=resolve.d.ts.map