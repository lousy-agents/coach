export declare const COMPILER_MODULE_FLAG_PREFIX = "--compiler-module=";
export declare const NATIVE_PACKAGE_FLAG_PREFIX = "--native-package=";
/**
 * An argv flag rather than a wire Request field, so
 * internal/projectbridge/protocol.go's frozen Request/Response shape stays
 * untouched.
 */
export declare function resolveCompilerRootURL(argv: readonly string[]): Promise<URL>;
export declare function resolveNativeTsserverPath(argv: readonly string[]): Promise<string>;
//# sourceMappingURL=compiler-argv.d.ts.map