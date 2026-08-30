/**
 * Minimal package.json "exports" resolution: a plain string (only for
 * subpath "."), a subpath-keyed object (with one-level condition objects
 * and a single "*" wildcard per pattern), or -- absent "exports" -- the
 * "main" field for subpath ".".
 */
export declare function resolvePackageExports(exp: unknown, main: string | undefined, subpath: string): string | undefined;
//# sourceMappingURL=resolve-exports.d.ts.map