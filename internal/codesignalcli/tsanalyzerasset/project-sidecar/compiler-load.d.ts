import type { CompilerBundle } from "./analyze.js";
/** Thrown for a compiler module that failed to resolve/load/declare the
 * required unstable API surface, or for a missing/invalid --native-package
 * argument. Distinct from SidecarBackendError, which covers a loaded
 * compiler whose native backend then fails to spawn. */
export declare class CompilerLoadError extends Error {
}
/**
 * Dynamically loads the three `typescript/unstable/*` subpaths this
 * sidecar needs directly from `rootURL`'s own package.json `exports` map,
 * rather than via bare-specifier resolution (which Node would always
 * satisfy from this package's own node_modules, regardless of rootURL).
 * A subpath missing from `exports`, or a resolved module missing a
 * required named export, is reported as a CompilerLoadError -- a
 * qualified incomplete report the caller turns into a structured Response
 * error, not a crash.
 */
export declare function loadCompiler(rootURL: URL): Promise<CompilerBundle>;
//# sourceMappingURL=compiler-load.d.ts.map